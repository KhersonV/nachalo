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
        groupId?: number;
        name: string;
        avatar?: string;
        characterType?: string;
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
    title: string;
    tone: OutcomeTone;
    isEscaped: boolean;
    isEliminated: boolean;
    isSurvived: boolean;
    playerWon?: boolean;
};

type SummaryItem = {
    label: string;
    value: string;
};

type HighlightItem = {
    label: string;
    value: string;
    detail: string;
};

type MatchBadge = {
    name: string;
    detail: string;
};

const BADGE_THRESHOLDS = {
    MONSTER_HUNTER_KILLS: 1,
    RESOURCE_COLLECTOR_REWARD_TOTAL: 25,
    CAREFUL_SURVIVOR_DAMAGE_TAKEN_MAX: 15,
    HEAVY_HITTER_DAMAGE: 40,
} as const;

const MAX_VISIBLE_BADGES = 5;

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

function statValue(value: unknown): number {
    return isFiniteStat(value) ? value : 0;
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

function totalRewardsAmount(rewards: Reward[]): number {
    return rewards.reduce(
        (sum, reward) => sum + statValue(reward.amount),
        0,
    );
}

function getWinnerText(stats: PlayerStatsPayload): string {
    if (stats.winnerId <= 0) return "Winner not determined";
    if (stats.winnerType === "group") return `Winning team: ${stats.winnerId}`;
    if (stats.winnerId === stats.player.userId) return "Winner: You";
    return `Winner: player ${stats.winnerId}`;
}

function getPlayerWon(stats: PlayerStatsPayload): boolean | undefined {
    if (typeof stats.player.isWinner === "boolean") {
        return stats.player.isWinner;
    }

    if (stats.winnerType === "user" && stats.winnerId > 0) {
        return stats.winnerId === stats.player.userId;
    }

    if (
        stats.winnerType === "group" &&
        stats.winnerId > 0 &&
        isFiniteStat(stats.player.groupId)
    ) {
        return stats.winnerId === stats.player.groupId;
    }

    return undefined;
}

function getOutcomeDisplay(
    stats: PlayerStatsPayload,
    flags: ResultFlags,
): OutcomeDisplay {
    const playerWon = getPlayerWon(stats);
    const hasDeaths = isFiniteStat(stats.player.deaths) && stats.player.deaths > 0;
    const isEscaped = flags.escaped || stats.player.escaped === true;
    const isSurvived = stats.player.survived === true;
    const isEliminated =
        flags.eliminated || stats.player.survived === false || hasDeaths;

    if (isEscaped) {
        return {
            title: "You Escaped!",
            tone: "win",
            isEscaped,
            isEliminated: false,
            isSurvived: true,
            playerWon,
        };
    }

    if (isEliminated) {
        return {
            title: "You Were Eliminated",
            tone: "loss",
            isEscaped: false,
            isEliminated,
            isSurvived: false,
            playerWon,
        };
    }

    if (playerWon === true) {
        return {
            title: "Victory!",
            tone: "win",
            isEscaped: false,
            isEliminated: false,
            isSurvived: true,
            playerWon,
        };
    }

    if (isSurvived) {
        return {
            title: "You Survived",
            tone: "neutral",
            isEscaped: false,
            isEliminated: false,
            isSurvived,
            playerWon,
        };
    }

    if (playerWon === false) {
        return {
            title: "Defeat",
            tone: "loss",
            isEscaped: false,
            isEliminated: false,
            isSurvived: false,
            playerWon,
        };
    }

    return {
        title: "Match Complete",
        tone: "neutral",
        isEscaped: false,
        isEliminated: false,
        isSurvived: false,
        playerWon,
    };
}

function formatKillCount(count: number): string {
    if (count === 1) return "1 defeat";
    return `${formatStatNumber(count)} defeats`;
}

function buildKillDetail(stats: PlayerStatsPayload): string {
    const playerKills = statValue(stats.player.playerKills);
    const monsterKills = statValue(stats.player.monsterKills);

    if (playerKills === 0 && monsterKills === 0) {
        return "Next run target: one confirmed takedown.";
    }

    const parts: string[] = [];
    if (monsterKills > 0) {
        parts.push(`${formatStatNumber(monsterKills)} monster`);
    }
    if (playerKills > 0) {
        parts.push(`${formatStatNumber(playerKills)} player`);
    }
    return parts.join(", ");
}

function buildMatchHighlights(
    stats: PlayerStatsPayload,
    rewards: Reward[],
    outcome: OutcomeDisplay,
): HighlightItem[] {
    const playerKills = statValue(stats.player.playerKills);
    const monsterKills = statValue(stats.player.monsterKills);
    const totalKills = playerKills + monsterKills;
    const damageTotal = statValue(stats.player.damageTotal);
    const damageToPlayers = statValue(stats.player.damageToPlayers);
    const damageToMonsters = statValue(stats.player.damageToMonsters);
    const damageTaken = stats.player.damageTaken;

    const resultDetail = outcome.isEscaped
        ? "Portal exit secured."
        : outcome.playerWon
          ? "You finished on top."
          : outcome.isEliminated
            ? "Run ended, but the next one starts clean."
            : outcome.isSurvived
              ? "You made it through the match."
              : "Final state recorded.";

    const highlights: HighlightItem[] = [
        {
            label: "Result",
            value: outcome.title,
            detail: resultDetail,
        },
        {
            label: "Combat",
            value: formatKillCount(totalKills),
            detail: buildKillDetail(stats),
        },
        {
            label: "Damage",
            value: formatStatNumber(damageTotal),
            detail:
                damageToPlayers > 0 || damageToMonsters > 0
                    ? `${formatStatNumber(damageToPlayers)} to players, ${formatStatNumber(damageToMonsters)} to monsters.`
                    : "Look for a cleaner opening fight next run.",
        },
        {
            label: "Spoils",
            value:
                rewards.length > 0
                    ? formatStatNumber(totalRewardsAmount(rewards))
                    : "None",
            detail:
                rewards.length > 0
                    ? formatRewardsSummary(rewards)
                    : "Escape, survive, and fight to earn more.",
        },
    ];

    if (isFiniteStat(damageTaken)) {
        highlights.push({
            label: "Defense",
            value: formatStatNumber(damageTaken),
            detail:
                damageTaken <= BADGE_THRESHOLDS.CAREFUL_SURVIVOR_DAMAGE_TAKEN_MAX
                    ? "Low damage taken."
                    : "Try taking less damage next time.",
        });
    }

    return highlights;
}

function buildMatchBadges(
    stats: PlayerStatsPayload,
    rewards: Reward[],
    outcome: OutcomeDisplay,
): MatchBadge[] {
    const monsterKills = statValue(stats.player.monsterKills);
    const totalRewards = totalRewardsAmount(rewards);
    const damageTotal = statValue(stats.player.damageTotal);
    const damageTaken = stats.player.damageTaken;
    const survivedMatch =
        outcome.isEscaped || outcome.isSurvived || outcome.playerWon === true;
    const carefulSurvival =
        survivedMatch &&
        isFiniteStat(damageTaken) &&
        damageTaken <= BADGE_THRESHOLDS.CAREFUL_SURVIVOR_DAMAGE_TAKEN_MAX;

    const badges: MatchBadge[] = [];

    if (outcome.isEscaped) {
        badges.push({
            name: "Successful Escape",
            detail: "Escaped through the portal this match.",
        });
    }

    if (survivedMatch) {
        badges.push({
            name: "Survivor",
            detail: "Finished the match without elimination.",
        });
    }

    if (monsterKills >= BADGE_THRESHOLDS.MONSTER_HUNTER_KILLS) {
        badges.push({
            name: "Monster Hunter",
            detail: `${formatStatNumber(monsterKills)} monster defeat${monsterKills === 1 ? "" : "s"}.`,
        });
    }

    if (damageTotal >= BADGE_THRESHOLDS.HEAVY_HITTER_DAMAGE) {
        badges.push({
            name: "Heavy Hitter",
            detail: `${formatStatNumber(damageTotal)} total damage.`,
        });
    }

    if (carefulSurvival) {
        badges.push({
            name: "Careful Survivor",
            detail: isFiniteStat(damageTaken)
                ? `${formatStatNumber(damageTaken)} damage taken.`
                : "No recorded deaths.",
        });
    }

    if (totalRewards >= BADGE_THRESHOLDS.RESOURCE_COLLECTOR_REWARD_TOTAL) {
        badges.push({
            name: "Resource Collector",
            detail: `${formatStatNumber(totalRewards)} total rewards.`,
        });
    }

    return badges.slice(0, MAX_VISIBLE_BADGES);
}

function buildReplayPrompt(
    stats: PlayerStatsPayload,
    outcome: OutcomeDisplay,
): string {
    const monsterKills = statValue(stats.player.monsterKills);
    const damageTotal = statValue(stats.player.damageTotal);
    const damageTaken = stats.player.damageTaken;

    if (outcome.isEscaped) {
        if (isFiniteStat(damageTaken) && damageTaken > 0) {
            return "Run it back and try escaping with less damage taken.";
        }
        return "Run it back and try to escape faster next time.";
    }

    if (outcome.isEliminated || outcome.tone === "loss") {
        if (isFiniteStat(damageTaken) && damageTaken > 0) {
            return "Try taking less damage next time.";
        }
        if (damageTotal < BADGE_THRESHOLDS.HEAVY_HITTER_DAMAGE) {
            return "Try landing a heavier opening fight next time.";
        }
        return "Try another class for a different strategy.";
    }

    if (monsterKills === 0) {
        return "Try hunting one monster before the portal next time.";
    }

    if (damageTotal >= BADGE_THRESHOLDS.HEAVY_HITTER_DAMAGE) {
        return "You hit hard. Run it back and beat that damage total.";
    }

    return "Try another class for a different strategy.";
}

function clearLastMatchSession() {
    if (typeof window === "undefined") return;
    sessionStorage.removeItem("lastMatchPlayerStats");
    sessionStorage.removeItem("lastMatchEscaped");
    sessionStorage.removeItem("lastMatchEliminated");
    sessionStorage.removeItem("lastMatchStatsPending");
    sessionStorage.removeItem("lastMatchPendingInstanceId");
    sessionStorage.removeItem("lastMatchEndedNoStats");
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
        value: `+${formatStatNumber(statValue(stats.player.expGained))}`,
    });

    summary.push({
        label: "Rewards",
        value: formatRewardsSummary(rewards),
    });

    summary.push({
        label: "Kills",
        value: formatStatNumber(
            statValue(stats.player.playerKills) +
                statValue(stats.player.monsterKills),
        ),
    });

    summary.push({
        label: "Damage",
        value: formatStatNumber(statValue(stats.player.damageTotal)),
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
    const highlights = useMemo(
        () =>
            stats && outcome
                ? buildMatchHighlights(stats, rewards, outcome)
                : [],
        [outcome, rewards, stats],
    );
    const badges = useMemo(
        () => (stats && outcome ? buildMatchBadges(stats, rewards, outcome) : []),
        [outcome, rewards, stats],
    );
    const replayPrompt = useMemo(
        () => (stats && outcome ? buildReplayPrompt(stats, outcome) : ""),
        [outcome, stats],
    );

    if (!stats && isPendingAfterDefeat) {
        return (
            <main className={styles.page}>
                <section className={styles.card}>
                    <p className={styles.eyebrow}>Match Result</p>
                    <h1 className={`${styles.title} ${styles.titleLoss}`}>
                        You Were Eliminated
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
                        {matchEndedWithoutStats && (
                            <button
                                type="button"
                                className={`${styles.button} ${styles.buttonPrimary}`}
                                onClick={() => {
                                    clearLastMatchSession();
                                    router.replace("/mode");
                                }}
                            >
                                Play Again
                            </button>
                        )}
                        <button
                            type="button"
                            className={`${styles.button} ${styles.buttonGhost}`}
                            onClick={() => {
                                clearLastMatchSession();
                                router.replace("/base");
                            }}
                        >
                            Back to Lobby
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
                    {replayPrompt && (
                        <p className={styles.replayPrompt}>{replayPrompt}</p>
                    )}
                </header>

                <section className={styles.replaySection}>
                    <div className={styles.sectionHeader}>
                        <h3 className={styles.sectionTitle}>Match Highlights</h3>
                    </div>
                    <div className={styles.highlightsGrid}>
                        {highlights.map((item) => (
                            <article
                                className={styles.highlightItem}
                                key={`${item.label}-${item.value}`}
                            >
                                <span className={styles.highlightLabel}>
                                    {item.label}
                                </span>
                                <strong className={styles.highlightValue}>
                                    {item.value}
                                </strong>
                                <span className={styles.highlightDetail}>
                                    {item.detail}
                                </span>
                            </article>
                        ))}
                    </div>
                </section>

                <section className={styles.replaySection}>
                    <div className={styles.sectionHeader}>
                        <h3 className={styles.sectionTitle}>Run Badges</h3>
                        <span className={styles.sectionMeta}>
                            Local to this result
                        </span>
                    </div>
                    {badges.length > 0 ? (
                        <div className={styles.badgeList}>
                            {badges.map((badge) => (
                                <article
                                    className={styles.badge}
                                    key={`${badge.name}-${badge.detail}`}
                                >
                                    <strong className={styles.badgeName}>
                                        {badge.name}
                                    </strong>
                                    <span className={styles.badgeDetail}>
                                        {badge.detail}
                                    </span>
                                </article>
                            ))}
                        </div>
                    ) : (
                        <p className={styles.badgeEmpty}>
                            No badges this match. One escape, one monster kill,
                            or {BADGE_THRESHOLDS.HEAVY_HITTER_DAMAGE} damage can
                            light up the next run.
                        </p>
                    )}
                </section>

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
                            +{formatStatNumber(statValue(stats.player.expGained))}
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
                            {formatStatNumber(statValue(stats.player.playerKills))}
                        </span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Monster Kills</span>
                        <span className={styles.value}>
                            {formatStatNumber(statValue(stats.player.monsterKills))}
                        </span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Total Damage</span>
                        <span className={styles.value}>
                            {formatStatNumber(statValue(stats.player.damageTotal))}
                        </span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Damage to Players</span>
                        <span className={styles.value}>
                            {formatStatNumber(statValue(stats.player.damageToPlayers))}
                        </span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Damage to Monsters</span>
                        <span className={styles.value}>
                            {formatStatNumber(statValue(stats.player.damageToMonsters))}
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
                        onClick={() => {
                            clearLastMatchSession();
                            router.replace("/mode");
                        }}
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
                        onClick={() => {
                            clearLastMatchSession();
                            router.replace("/base");
                        }}
                    >
                        Back to Lobby
                    </button>
                </div>
            </section>
        </main>
    );
}
