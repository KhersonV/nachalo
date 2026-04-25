"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { API_BASE, WS_URL } from "@/utils/serviceUrls";
import { rewardLabel } from "@/utils/matchHistory";
import styles from "@/styles/GameStatsPage.module.css";

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
        damageTaken?: number;
        placement?: number;
        survived?: boolean;
        deaths?: number;
        isWinner?: boolean;
        escaped?: boolean;
        rewards?: Reward[] | Record<string, number> | string | null;
    };
};

type ResultFlags = {
    escaped: boolean;
    eliminated: boolean;
};

type OutcomeTone = "win" | "loss" | "neutral";

type OutcomeDisplay = {
    title: "Victory" | "Defeat" | "Eliminated" | "Escaped" | "Match Complete";
    tone: OutcomeTone;
    isEscaped: boolean;
    isEliminated: boolean;
};

type SummaryItem = {
    label: string;
    value: string;
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

function readResultFlags(): ResultFlags {
    if (typeof window === "undefined") {
        return { escaped: false, eliminated: false };
    }

    return {
        escaped: sessionStorage.getItem("lastMatchEscaped") === "1",
        eliminated:
            sessionStorage.getItem("lastMatchEliminated") === "1" ||
            sessionStorage.getItem("lastMatchStatsPending") === "1",
    };
}

function isFiniteStat(value: unknown): value is number {
    return typeof value === "number" && Number.isFinite(value);
}

function formatStatNumber(value: number): string {
    return new Intl.NumberFormat().format(value);
}

function formatReward(reward: Reward): string {
    const label = rewardLabel(reward.type ?? "");
    return `${label}: ${formatStatNumber(reward.amount ?? 0)}`;
}

function formatRewardsSummary(rewards: Reward[]): string {
    if (rewards.length === 0) return "No rewards";
    return rewards.map((reward) => formatReward(reward)).join(", ");
}

function getWinnerText(stats: PlayerStatsPayload): string {
    if (stats.winnerId <= 0) return "Winner not determined";
    if (stats.winnerType === "group") return `Winning team: ${stats.winnerId}`;
    if (stats.winnerId === stats.player.userId) return "Winner: You";
    return `Winner: player ${stats.winnerId}`;
}

function getOutcomeDisplay(
    stats: PlayerStatsPayload,
    flags: ResultFlags,
): OutcomeDisplay {
    const playerWon =
        typeof stats.player.isWinner === "boolean"
            ? stats.player.isWinner
            : stats.winnerType === "user" && stats.winnerId > 0
              ? stats.winnerId === stats.player.userId
              : undefined;
    const hasDeaths = isFiniteStat(stats.player.deaths) && stats.player.deaths > 0;
    const isEscaped = flags.escaped || stats.player.escaped === true;
    const isEliminated =
        flags.eliminated || stats.player.survived === false || hasDeaths;

    if (isEscaped) {
        return {
            title: "Escaped",
            tone: "win",
            isEscaped,
            isEliminated: false,
        };
    }

    if (isEliminated) {
        return {
            title: "Eliminated",
            tone: "loss",
            isEscaped: false,
            isEliminated,
        };
    }

    if (playerWon === true) {
        return {
            title: "Victory",
            tone: "win",
            isEscaped: false,
            isEliminated: false,
        };
    }

    if (playerWon === false) {
        return {
            title: "Defeat",
            tone: "loss",
            isEscaped: false,
            isEliminated: false,
        };
    }

    return {
        title: "Match Complete",
        tone: "neutral",
        isEscaped: false,
        isEliminated: false,
    };
}

function buildResultSummary(
    stats: PlayerStatsPayload,
    rewards: Reward[],
    outcome: OutcomeDisplay,
): SummaryItem[] {
    const summary: SummaryItem[] = [
        { label: "Winner", value: getWinnerText(stats) },
    ];

    if (isFiniteStat(stats.player.placement)) {
        summary.push({
            label: "Placement",
            value: `#${stats.player.placement}`,
        });
    }

    if (outcome.isEscaped) {
        summary.push({ label: "Exit", value: "Escaped" });
    } else if (typeof stats.player.survived === "boolean") {
        summary.push({
            label: "Survival",
            value: stats.player.survived ? "Survived" : "Eliminated",
        });
    } else if (outcome.isEliminated) {
        summary.push({ label: "Survival", value: "Eliminated" });
    }

    if (isFiniteStat(stats.player.deaths)) {
        summary.push({
            label: "Deaths",
            value: formatStatNumber(stats.player.deaths),
        });
    }

    summary.push({
        label: "XP",
        value: `+${formatStatNumber(stats.player.expGained)}`,
    });

    summary.push({
        label: "Rewards",
        value: formatRewardsSummary(rewards),
    });

    summary.push({
        label: "Kills",
        value: formatStatNumber(
            stats.player.playerKills + stats.player.monsterKills,
        ),
    });

    summary.push({
        label: "Damage",
        value: formatStatNumber(stats.player.damageTotal),
    });

    return summary;
}

export default function GameStatsPage() {
    const router = useRouter();
    const [stats, setStats] = useState<PlayerStatsPayload | null>(null);
    const [isPendingAfterDefeat, setIsPendingAfterDefeat] = useState(false);
    const [matchEndedWithoutStats, setMatchEndedWithoutStats] = useState(false);
    const [resultFlags, setResultFlags] = useState<ResultFlags>({
        escaped: false,
        eliminated: false,
    });

    useEffect(() => {
        if (typeof window === "undefined") return;
        const tryLoadStats = () => {
            const raw = sessionStorage.getItem("lastMatchPlayerStats");
            if (!raw) return false;
            try {
                const parsed = JSON.parse(raw) as PlayerStatsPayload;
                if (!parsed?.player) return false;
                setResultFlags(readResultFlags());
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
            setResultFlags((current) => ({ ...current, eliminated: true }));
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
            setResultFlags(readResultFlags());
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
    const outcome = useMemo(
        () => (stats ? getOutcomeDisplay(stats, resultFlags) : null),
        [stats, resultFlags],
    );
    const resultSummary = useMemo(
        () => (stats && outcome ? buildResultSummary(stats, rewards, outcome) : []),
        [outcome, rewards, stats],
    );

    if (!stats && isPendingAfterDefeat) {
        return (
            <main className={styles.page}>
                <section className={styles.card}>
                    <p className={styles.eyebrow}>Match Result</p>
                    <h1 className={`${styles.title} ${styles.titleLoss}`}>
                        Eliminated
                    </h1>
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
                            type="button"
                            className={`${styles.button} ${styles.buttonGhost}`}
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

    const winnerText = getWinnerText(stats);
    const titleToneClass =
        outcome?.tone === "win"
            ? styles.titleWin
            : outcome?.tone === "loss"
              ? styles.titleLoss
              : styles.titleNeutral;

    return (
        <main className={styles.page}>
            <section className={styles.card}>
                <header className={styles.resultHeader}>
                    <p className={styles.eyebrow}>Match Result</p>
                    <h1 className={`${styles.title} ${titleToneClass}`}>
                        {outcome?.title ?? "Match Complete"}
                    </h1>
                    <p className={styles.subtitle}>{winnerText}</p>
                </header>

                <div className={styles.summaryGrid}>
                    {resultSummary.map((item) => (
                        <div
                            className={styles.summaryItem}
                            key={`${item.label}-${item.value}`}
                        >
                            <span className={styles.summaryLabel}>
                                {item.label}
                            </span>
                            <span className={styles.summaryValue}>
                                {item.value}
                            </span>
                        </div>
                    ))}
                </div>

                <div className={styles.playerBlock}>
                    <h2 className={styles.playerName}>{stats.player.name}</h2>
                    <p className={styles.instance}>Match: {stats.instanceId}</p>
                </div>

                <div className={styles.grid}>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Experience</span>
                        <span className={styles.value}>
                            +{formatStatNumber(stats.player.expGained)}
                        </span>
                    </div>
                    {isFiniteStat(stats.player.placement) && (
                        <div className={styles.statItem}>
                            <span className={styles.label}>Placement</span>
                            <span className={styles.value}>
                                #{stats.player.placement}
                            </span>
                        </div>
                    )}
                    {isFiniteStat(stats.player.deaths) && (
                        <div className={styles.statItem}>
                            <span className={styles.label}>Deaths</span>
                            <span className={styles.value}>
                                {formatStatNumber(stats.player.deaths)}
                            </span>
                        </div>
                    )}
                    <div className={styles.statItem}>
                        <span className={styles.label}>Player Kills</span>
                        <span className={styles.value}>
                            {formatStatNumber(stats.player.playerKills)}
                        </span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Monster Kills</span>
                        <span className={styles.value}>
                            {formatStatNumber(stats.player.monsterKills)}
                        </span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Total Damage</span>
                        <span className={styles.value}>
                            {formatStatNumber(stats.player.damageTotal)}
                        </span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Damage to Players</span>
                        <span className={styles.value}>
                            {formatStatNumber(stats.player.damageToPlayers)}
                        </span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Damage to Monsters</span>
                        <span className={styles.value}>
                            {formatStatNumber(stats.player.damageToMonsters)}
                        </span>
                    </div>
                    {isFiniteStat(stats.player.damageTaken) && (
                        <div className={styles.statItem}>
                            <span className={styles.label}>Damage Taken</span>
                            <span className={styles.value}>
                                {formatStatNumber(stats.player.damageTaken)}
                            </span>
                        </div>
                    )}
                    {typeof stats.player.survived === "boolean" && (
                        <div className={styles.statItem}>
                            <span className={styles.label}>Survival</span>
                            <span className={styles.value}>
                                {stats.player.survived
                                    ? "Survived"
                                    : "Eliminated"}
                            </span>
                        </div>
                    )}
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
                                    {formatReward(reward)}
                                </li>
                            ))}
                        </ul>
                    )}
                </section>

                <div className={styles.actions}>
                    <button
                        type="button"
                        className={`${styles.button} ${styles.buttonPrimary}`}
                        onClick={() => router.replace("/mode")}
                    >
                        Play Again
                    </button>
                    <button
                        type="button"
                        className={`${styles.button} ${styles.buttonSecondary}`}
                        onClick={() => router.replace("/matches")}
                    >
                        Match History
                    </button>
                    <button
                        type="button"
                        className={`${styles.button} ${styles.buttonGhost}`}
                        onClick={() => router.replace("/mode")}
                    >
                        Back to Modes
                    </button>
                </div>
            </section>
        </main>
    );
}
