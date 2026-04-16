"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { API_BASE } from "@/utils/serviceUrls";
import type { MatchHistoryListItem, MatchHistoryListResponse } from "@/types";
import {
    formatMatchFinishedAt,
    formatMatchMode,
    formatParticipantsSummary,
    formatResultStatus,
    formatWinnerSummary,
    formatRewardSummary,
} from "@/utils/matchHistory";
import { useAuth } from "../contexts/AuthContext";
import LobbyHeader from "./LobbyHeader";
import styles from "../styles/MatchHistoryPage.module.css";

export default function MatchHistoryPage() {
    const router = useRouter();
    const { user } = useAuth();
    const [matches, setMatches] = useState<MatchHistoryListItem[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");

    useEffect(() => {
        if (!user?.token) {
            return;
        }

        let cancelled = false;

        const loadMatchHistory = async () => {
            setLoading(true);
            setError("");

            try {
                const res = await fetch(`${API_BASE}/game/matches/history`, {
                    headers: {
                        Authorization: `Bearer ${user.token}`,
                    },
                    cache: "no-store",
                });

                if (!res.ok) {
                    throw new Error("Failed to load completed matches");
                }

                const payload: MatchHistoryListResponse = await res.json();
                if (cancelled) return;
                setMatches(payload.data ?? []);
            } catch (err: any) {
                if (cancelled) return;
                setError(err?.message || "Failed to load completed matches");
            } finally {
                if (!cancelled) {
                    setLoading(false);
                }
            }
        };

        void loadMatchHistory();

        return () => {
            cancelled = true;
        };
    }, [user]);

    return (
        <>
            <LobbyHeader />
            <main className={styles.page}>
                <section className={styles.topRow}>
                    <div>
                        <h2 className={styles.title}>Match History</h2>
                        <p className={styles.subtitle}>
                            Completed matches are stored here in reverse
                            chronological order.
                        </p>
                    </div>
                    <button
                        type="button"
                        className={styles.backBtn}
                        onClick={() => router.push("/mode")}
                    >
                        Back to Modes
                    </button>
                </section>

                {loading ? (
                    <section className={styles.stateCard}>
                        Loading completed matches...
                    </section>
                ) : null}

                {!loading && error ? (
                    <section className={styles.stateCard}>{error}</section>
                ) : null}

                {!loading && !error && matches.length === 0 ? (
                    <section className={styles.stateCard}>
                        You do not have any completed matches yet.
                    </section>
                ) : null}

                {!loading && !error && matches.length > 0 ? (
                    <div className={styles.list}>
                        {matches.map((match) => (
                            <button
                                type="button"
                                key={match.matchId}
                                className={styles.card}
                                onClick={() =>
                                    router.push(
                                        `/matches/${encodeURIComponent(match.matchId)}`,
                                    )
                                }
                            >
                                <div className={styles.cardHeader}>
                                    <div>
                                        <p className={styles.meta}>
                                            {formatMatchFinishedAt(
                                                match.finishedAt,
                                            )}
                                        </p>
                                        <h3 className={styles.cardTitle}>
                                            {formatMatchMode(match.mode)}
                                        </h3>
                                    </div>
                                    <span className={styles.statusBadge}>
                                        {match.status || "completed"}
                                    </span>
                                </div>

                                <div className={styles.infoGrid}>
                                    <div className={styles.infoBlock}>
                                        <span className={styles.label}>
                                            Participants
                                        </span>
                                        <span className={styles.valueText}>
                                            {formatParticipantsSummary(
                                                match.participants,
                                            )}
                                        </span>
                                    </div>
                                    <div className={styles.infoBlock}>
                                        <span className={styles.label}>
                                            Winner
                                        </span>
                                        <span className={styles.valueText}>
                                            {formatWinnerSummary(
                                                match.winner,
                                                match.participants,
                                            )}
                                        </span>
                                    </div>
                                    <div className={styles.infoBlock}>
                                        <span className={styles.label}>
                                            Your rewards
                                        </span>
                                        <span className={styles.valueText}>
                                            {match.playerRewardSummary.currencyGained}{" "}
                                            currency,{" "}
                                            {match.playerRewardSummary.expGained}{" "}
                                            exp
                                        </span>
                                        <span className={styles.subvalue}>
                                            {formatRewardSummary(
                                                match.playerRewardSummary.rewards,
                                            )}
                                        </span>
                                    </div>
                                    <div className={styles.infoBlock}>
                                        <span className={styles.label}>
                                            Your result
                                        </span>
                                        <span className={styles.valueText}>
                                            {formatResultStatus(
                                                match.playerResultSummary,
                                            )}
                                        </span>
                                        <span className={styles.subvalue}>
                                            Placement #{match.playerResultSummary.placement || "-"},
                                            {" "}
                                            Kills {match.playerResultSummary.kills},
                                            {" "}
                                            Deaths {match.playerResultSummary.deaths}
                                        </span>
                                    </div>
                                </div>
                            </button>
                        ))}
                    </div>
                ) : null}
            </main>
        </>
    );
}
