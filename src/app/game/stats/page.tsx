"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import styles from "@/styles/GameStatsPage.module.css";

const WS_URL = process.env.NEXT_PUBLIC_WS_URL || "ws://localhost:8001/ws";
const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8001";

type Reward = {
    type?: string;
    amount?: number;
};

type PlayerStatsPayload = {
    instanceId: string;
    winnerType: "user" | "group";
    winnerId: number;
    player: {
        userId: number;
        name: string;
        expGained: number;
        playerKills: number;
        monsterKills: number;
        damageTotal: number;
        damageToPlayers: number;
        damageToMonsters: number;
        rewards?: Reward[] | string | null;
    };
};

function parseRewards(raw: PlayerStatsPayload["player"]["rewards"]): Reward[] {
    if (!raw) return [];

    const normalizeRewardEntry = (entry: unknown): Reward | null => {
        if (!entry || typeof entry !== "object") return null;
        const rec = entry as Record<string, unknown>;
        const typeVal = rec.type ?? rec.Type;
        const amountVal = rec.amount ?? rec.Amount;
        if (typeof typeVal !== "string") return null;
        const amountNum = Number(amountVal ?? 0);
        if (!Number.isFinite(amountNum)) return null;
        return { type: typeVal, amount: amountNum };
    };

    if (Array.isArray(raw)) {
        return raw
            .map((r) => normalizeRewardEntry(r))
            .filter((r): r is Reward => r !== null);
    }

    const toRewardsFromObject = (obj: Record<string, unknown>): Reward[] =>
        Object.entries(obj)
            .filter(([, value]) => typeof value === "number")
            .map(([type, amount]) => ({
                type,
                amount: Number(amount),
            }));

    if (typeof raw === "string") {
        try {
            const parsed = JSON.parse(raw);
            if (Array.isArray(parsed)) {
                return parsed
                    .map((r) => normalizeRewardEntry(r))
                    .filter((r): r is Reward => r !== null);
            }
            if (parsed && typeof parsed === "object") {
                return toRewardsFromObject(parsed as Record<string, unknown>);
            }
            return [];
        } catch {
            return [];
        }
    }

    if (typeof raw === "object") {
        return toRewardsFromObject(raw as Record<string, unknown>);
    }

    return [];
}

export default function GameStatsPage() {
    const router = useRouter();
    const [stats, setStats] = useState<PlayerStatsPayload | null>(null);
    const [isPendingAfterDefeat, setIsPendingAfterDefeat] = useState(false);
    const [matchEndedWithoutStats, setMatchEndedWithoutStats] = useState(false);

    useEffect(() => {
        if (typeof window === "undefined") return;
        const tryLoadStats = () => {
            const raw = sessionStorage.getItem("lastMatchPlayerStats");
            if (!raw) return false;
            try {
                const parsed = JSON.parse(raw) as PlayerStatsPayload;
                if (!parsed?.player) return false;
                setStats(parsed);
                sessionStorage.removeItem("lastMatchStatsPending");
                setIsPendingAfterDefeat(false);
                return true;
            } catch {
                return false;
            }
        };

        if (tryLoadStats()) return;

        const hasPendingFlag =
            sessionStorage.getItem("lastMatchStatsPending") === "1";
        if (hasPendingFlag) {
            setIsPendingAfterDefeat(true);
        }
        if (sessionStorage.getItem("lastMatchEndedNoStats") === "1") {
            setMatchEndedWithoutStats(true);
        }

        const onStatsReady = () => {
            tryLoadStats();
        };

        window.addEventListener("match-stats-ready", onStatsReady);

        const redirectTimer = window.setTimeout(() => {
            if (!tryLoadStats() && !hasPendingFlag) {
                router.replace("/mode");
            }
        }, 1500);

        return () => {
            window.removeEventListener("match-stats-ready", onStatsReady);
            window.clearTimeout(redirectTimer);
        };
    }, [router]);

    useEffect(() => {
        if (typeof window === "undefined") return;
        if (!isPendingAfterDefeat || stats) return;

        const pendingInstanceId = sessionStorage.getItem(
            "lastMatchPendingInstanceId",
        );
        const storedUser = localStorage.getItem("user");
        const parsedUser = storedUser ? JSON.parse(storedUser) : null;
        const token: string | undefined = parsedUser?.token;
        const myUserId: number | undefined = parsedUser?.id;

        if (!pendingInstanceId || !token || !myUserId) {
            return;
        }

        let consecutiveApiMisses = 0;

        const applyStats = (prepared: PlayerStatsPayload) => {
            sessionStorage.setItem(
                "lastMatchPlayerStats",
                JSON.stringify(prepared),
            );
            sessionStorage.removeItem("lastMatchStatsPending");
            sessionStorage.removeItem("lastMatchPendingInstanceId");
            sessionStorage.removeItem("lastMatchEndedNoStats");
            setStats(prepared);
            setIsPendingAfterDefeat(false);
            setMatchEndedWithoutStats(false);
        };

        const tryLoadFromApi = async () => {
            try {
                const res = await fetch(
                    `${API_BASE}/game/match/${pendingInstanceId}/my-stats`,
                    {
                        method: "GET",
                        headers: {
                            Authorization: `Bearer ${token}`,
                        },
                        cache: "no-store",
                    },
                );
                if (!res.ok) {
                    consecutiveApiMisses += 1;
                    if (consecutiveApiMisses >= 8) {
                        sessionStorage.setItem("lastMatchEndedNoStats", "1");
                        setMatchEndedWithoutStats(true);
                    }
                    return false;
                }
                const data = await res.json();
                if (!data?.stats?.player) return false;
                consecutiveApiMisses = 0;
                applyStats(data.stats as PlayerStatsPayload);
                return true;
            } catch {
                consecutiveApiMisses += 1;
                if (consecutiveApiMisses >= 8) {
                    sessionStorage.setItem("lastMatchEndedNoStats", "1");
                    setMatchEndedWithoutStats(true);
                }
                return false;
            }
        };

        void tryLoadFromApi();
        const pollTimer = window.setInterval(() => {
            void tryLoadFromApi();
        }, 1500);

        let closed = false;
        const ws = new WebSocket(
            `${WS_URL}?token=${token}&instanceId=${pendingInstanceId}`,
        );

        ws.onopen = () => {
            if (closed) return;
            ws.send(
                JSON.stringify({
                    type: "JOIN_MATCH",
                    instanceId: pendingInstanceId,
                }),
            );
        };

        ws.onmessage = (event) => {
            try {
                const msg = JSON.parse(event.data);
                if (msg?.type !== "MATCH_ENDED") return;

                const allStats = Array.isArray(msg?.payload?.stats)
                    ? msg.payload.stats
                    : [];
                const myStats = allStats.find(
                    (s: any) => s?.userId === myUserId,
                );

                if (myStats) {
                    const prepared: PlayerStatsPayload = {
                        instanceId:
                            msg.payload.instanceId ??
                            msg.payload.instance_id ??
                            pendingInstanceId,
                        winnerType: msg.payload.winnerType ?? "user",
                        winnerId: msg.payload.winnerId ?? 0,
                        player: myStats,
                    };
                    applyStats(prepared);
                    return;
                }

                void tryLoadFromApi();
            } catch {
                // ignore malformed WS payloads
            }
        };

        return () => {
            closed = true;
            window.clearInterval(pollTimer);
            ws.close(1000);
        };
    }, [isPendingAfterDefeat, stats]);

    const rewards = useMemo(() => parseRewards(stats?.player.rewards), [stats]);

    if (!stats && isPendingAfterDefeat) {
        return (
            <main className={styles.page}>
                <section className={styles.card}>
                    <h1 className={styles.title}>You Died</h1>
                    {matchEndedWithoutStats ? (
                        <p className={styles.subtitle}>
                            Match ended. Detailed statistics for this player are
                            unavailable.
                        </p>
                    ) : (
                        <p className={styles.subtitle}>
                            Final statistics will appear after the match ends.
                        </p>
                    )}
                    <div className={styles.actions}>
                        <button
                            className={styles.button}
                            onClick={() => router.replace("/mode")}
                        >
                            Back to Modes
                        </button>
                    </div>
                </section>
            </main>
        );
    }

    if (!stats) {
        return <div className={styles.loading}>Loading statistics...</div>;
    }

    const winnerText =
        stats.winnerId > 0
            ? stats.winnerType === "group"
                ? `Winning team: ${stats.winnerId}`
                : `Winner: player ${stats.winnerId}`
            : "Winner not determined";

    return (
        <main className={styles.page}>
            <section className={styles.card}>
                <h1 className={styles.title}>Match Statistics</h1>
                <p className={styles.subtitle}>{winnerText}</p>

                <div className={styles.playerBlock}>
                    <h2 className={styles.playerName}>{stats.player.name}</h2>
                    <p className={styles.instance}>Match: {stats.instanceId}</p>
                </div>

                <div className={styles.grid}>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Experience</span>
                        <span className={styles.value}>
                            {stats.player.expGained}
                        </span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Player Kills</span>
                        <span className={styles.value}>
                            {stats.player.playerKills}
                        </span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Monster Kills</span>
                        <span className={styles.value}>
                            {stats.player.monsterKills}
                        </span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Total Damage</span>
                        <span className={styles.value}>
                            {stats.player.damageTotal}
                        </span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Damage to Players</span>
                        <span className={styles.value}>
                            {stats.player.damageToPlayers}
                        </span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Damage to Monsters</span>
                        <span className={styles.value}>
                            {stats.player.damageToMonsters}
                        </span>
                    </div>
                </div>

                <section className={styles.rewards}>
                    <h3 className={styles.rewardsTitle}>Rewards</h3>
                    {rewards.length === 0 ? (
                        <p className={styles.rewardsEmpty}>
                            No rewards received
                        </p>
                    ) : (
                        <ul className={styles.rewardsList}>
                            {rewards.map((reward, idx) => (
                                <li
                                    key={`${reward.type ?? "reward"}-${idx}`}
                                    className={styles.rewardItem}
                                >
                                    {(reward.type === "balance" ||
                                    reward.type === "coin" ||
                                    reward.type === "coins"
                                        ? "Balance"
                                        : (reward.type ?? "unknown")) +
                                        ":"} {" "}
                                    {reward.amount ?? 0}
                                </li>
                            ))}
                        </ul>
                    )}
                </section>

                <div className={styles.actions}>
                    <button
                        className={styles.button}
                        onClick={() => router.replace("/mode")}
                    >
                        Back to Modes
                    </button>
                </div>
            </section>
        </main>
    );
}
