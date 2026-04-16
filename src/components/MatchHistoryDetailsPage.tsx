"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { API_BASE } from "@/utils/serviceUrls";
import type {
    MatchHistoryDetailItem,
    MatchHistoryDetailResponse,
} from "@/types";
import {
    formatMatchFinishedAt,
    formatMatchMode,
    formatParticipantsSummary,
    formatResultStatus,
    formatWinnerSummary,
    rewardLabel,
} from "@/utils/matchHistory";
import { useAuth } from "../contexts/AuthContext";
import LobbyHeader from "./LobbyHeader";
import styles from "../styles/MatchHistoryDetailsPage.module.css";

type MatchHistoryDetailsPageProps = {
    instanceId: string;
};

function resolveErrorMessage(errorCode: string): string {
    switch (errorCode) {
        case "match_not_completed":
            return "This match is still in progress. Final statistics become available only after full completion.";
        case "match_access_denied":
            return "You can only view statistics for matches where you participated.";
        case "match_not_found":
            return "Match statistics were not found.";
        default:
            return "Failed to load match statistics.";
    }
}

export default function MatchHistoryDetailsPage({
    instanceId,
}: MatchHistoryDetailsPageProps) {
    const router = useRouter();
    const { user } = useAuth();
    const [details, setDetails] = useState<MatchHistoryDetailItem | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");

    useEffect(() => {
        if (!user?.token || !instanceId) {
            return;
        }

        let cancelled = false;

        const loadMatchDetails = async () => {
            setLoading(true);
            setError("");

            try {
                const res = await fetch(
                    `${API_BASE}/game/matches/history/${encodeURIComponent(instanceId)}`,
                    {
                        headers: {
                            Authorization: `Bearer ${user.token}`,
                        },
                        cache: "no-store",
                    },
                );

                if (!res.ok) {
                    let errorCode = "failed_to_load_match_details";
                    try {
                        const payload = await res.json();
                        if (typeof payload?.error === "string") {
                            errorCode = payload.error;
                        }
                    } catch {
                        // ignore malformed error payloads
                    }
                    throw new Error(resolveErrorMessage(errorCode));
                }

                const payload: MatchHistoryDetailResponse = await res.json();
                if (cancelled) return;
                setDetails(payload.data);
            } catch (err: any) {
                if (cancelled) return;
                setError(err?.message || "Failed to load match statistics.");
            } finally {
                if (!cancelled) {
                    setLoading(false);
                }
            }
        };

        void loadMatchDetails();

        return () => {
            cancelled = true;
        };
    }, [instanceId, user]);

    return (
        <>
            <LobbyHeader />
            <main className={styles.page}>
                <section className={styles.topRow}>
                    <div>
                        <h2 className={styles.title}>Match Details</h2>
                        <p className={styles.subtitle}>
                            Final personal statistics for the selected match.
                        </p>
                    </div>
                    <button
                        type="button"
                        className={styles.backBtn}
                        onClick={() => router.push("/matches")}
                    >
                        Back to History
                    </button>
                </section>

                {loading ? (
                    <section className={styles.stateCard}>
                        Loading match statistics...
                    </section>
                ) : null}

                {!loading && error ? (
                    <section className={styles.stateCard}>{error}</section>
                ) : null}

                {!loading && !error && details ? (
                    <section className={styles.card}>
                        <div className={styles.hero}>
                            <div>
                                <p className={styles.meta}>
                                    {formatMatchFinishedAt(details.finishedAt)}
                                </p>
                                <h3 className={styles.heroTitle}>
                                    {formatMatchMode(details.mode)}
                                </h3>
                                <p className={styles.heroSubtitle}>
                                    {formatWinnerSummary(
                                        details.winner,
                                        details.participants,
                                    )}
                                </p>
                            </div>
                            <span className={styles.statusBadge}>
                                {formatResultStatus(details.playerResultSummary)}
                            </span>
                        </div>

                        <div className={styles.sectionGrid}>
                            <section className={styles.section}>
                                <h4 className={styles.sectionTitle}>
                                    Match snapshot
                                </h4>
                                <div className={styles.kvList}>
                                    <div className={styles.kvItem}>
                                        <span className={styles.label}>
                                            Match ID
                                        </span>
                                        <span className={styles.value}>
                                            {details.matchId}
                                        </span>
                                    </div>
                                    <div className={styles.kvItem}>
                                        <span className={styles.label}>
                                            Participants
                                        </span>
                                        <span className={styles.valueText}>
                                            {formatParticipantsSummary(
                                                details.participants,
                                            )}
                                        </span>
                                    </div>
                                    <div className={styles.kvItem}>
                                        <span className={styles.label}>
                                            Placement
                                        </span>
                                        <span className={styles.value}>
                                            #{details.playerResultSummary.placement || "-"}
                                        </span>
                                    </div>
                                    <div className={styles.kvItem}>
                                        <span className={styles.label}>
                                            Survival
                                        </span>
                                        <span className={styles.value}>
                                            {details.currentPlayerStats.survived
                                                ? "Survived"
                                                : "Defeated"}
                                        </span>
                                    </div>
                                </div>
                            </section>

                            <section className={styles.section}>
                                <h4 className={styles.sectionTitle}>
                                    Rewards
                                </h4>
                                <div className={styles.kvList}>
                                    <div className={styles.kvItem}>
                                        <span className={styles.label}>
                                            Experience gained
                                        </span>
                                        <span className={styles.value}>
                                            {details.currentPlayerStats.expGained}
                                        </span>
                                    </div>
                                    <div className={styles.kvItem}>
                                        <span className={styles.label}>
                                            Currency gained
                                        </span>
                                        <span className={styles.value}>
                                            {details.currentPlayerStats.currencyGained}
                                        </span>
                                    </div>
                                </div>
                                {details.currentPlayerStats.rewards.length > 0 ? (
                                    <ul className={styles.list}>
                                        {details.currentPlayerStats.rewards.map(
                                            (reward, index) => (
                                                <li
                                                    key={`${reward.type}-${index}`}
                                                    className={styles.listItem}
                                                >
                                                    {rewardLabel(reward.type)}:{" "}
                                                    {reward.amount}
                                                </li>
                                            ),
                                        )}
                                    </ul>
                                ) : (
                                    <p className={styles.emptyText}>
                                        No final rewards recorded.
                                    </p>
                                )}
                            </section>

                            <section className={styles.section}>
                                <h4 className={styles.sectionTitle}>Combat</h4>
                                <div className={styles.kvList}>
                                    <div className={styles.kvItem}>
                                        <span className={styles.label}>
                                            Damage dealt
                                        </span>
                                        <span className={styles.value}>
                                            {details.currentPlayerStats.damageDealt}
                                        </span>
                                    </div>
                                    <div className={styles.kvItem}>
                                        <span className={styles.label}>
                                            Damage taken
                                        </span>
                                        <span className={styles.value}>
                                            {details.currentPlayerStats.damageTaken}
                                        </span>
                                    </div>
                                    <div className={styles.kvItem}>
                                        <span className={styles.label}>
                                            Damage to players
                                        </span>
                                        <span className={styles.value}>
                                            {
                                                details.currentPlayerStats
                                                    .damageToPlayers
                                            }
                                        </span>
                                    </div>
                                    <div className={styles.kvItem}>
                                        <span className={styles.label}>
                                            Damage to monsters
                                        </span>
                                        <span className={styles.value}>
                                            {
                                                details.currentPlayerStats
                                                    .damageToMonsters
                                            }
                                        </span>
                                    </div>
                                </div>
                            </section>

                            <section className={styles.section}>
                                <h4 className={styles.sectionTitle}>
                                    Eliminations
                                </h4>
                                <div className={styles.kvList}>
                                    <div className={styles.kvItem}>
                                        <span className={styles.label}>
                                            Total kills
                                        </span>
                                        <span className={styles.value}>
                                            {details.currentPlayerStats.kills}
                                        </span>
                                    </div>
                                    <div className={styles.kvItem}>
                                        <span className={styles.label}>
                                            Player kills
                                        </span>
                                        <span className={styles.value}>
                                            {details.currentPlayerStats.playerKills}
                                        </span>
                                    </div>
                                    <div className={styles.kvItem}>
                                        <span className={styles.label}>
                                            Monster kills
                                        </span>
                                        <span className={styles.value}>
                                            {details.currentPlayerStats.monsterKills}
                                        </span>
                                    </div>
                                    <div className={styles.kvItem}>
                                        <span className={styles.label}>
                                            Deaths
                                        </span>
                                        <span className={styles.value}>
                                            {details.currentPlayerStats.deaths}
                                        </span>
                                    </div>
                                </div>
                            </section>

                            <section className={styles.section}>
                                <h4 className={styles.sectionTitle}>
                                    Artifacts
                                </h4>
                                {details.currentPlayerStats.artifacts.length > 0 ? (
                                    <ul className={styles.list}>
                                        {details.currentPlayerStats.artifacts.map(
                                            (artifact) => (
                                                <li
                                                    key={`${artifact.itemId}-${artifact.name}`}
                                                    className={styles.listItem}
                                                >
                                                    {artifact.name}
                                                    {artifact.count > 1
                                                        ? ` x${artifact.count}`
                                                        : ""}
                                                    {artifact.description
                                                        ? ` - ${artifact.description}`
                                                        : ""}
                                                </li>
                                            ),
                                        )}
                                    </ul>
                                ) : (
                                    <p className={styles.emptyText}>
                                        No final artifacts were recorded for this
                                        player.
                                    </p>
                                )}
                            </section>

                            <section className={styles.section}>
                                <h4 className={styles.sectionTitle}>
                                    Bonuses
                                </h4>
                                {details.currentPlayerStats.otherBonuses.length >
                                0 ? (
                                    <ul className={styles.list}>
                                        {details.currentPlayerStats.otherBonuses.map(
                                            (bonus) => (
                                                <li
                                                    key={bonus.type}
                                                    className={styles.listItem}
                                                >
                                                    {bonus.label}
                                                    {bonus.amount
                                                        ? `: ${bonus.amount} currency`
                                                        : ""}
                                                    {bonus.expBonus
                                                        ? `, ${bonus.expBonus} exp`
                                                        : ""}
                                                </li>
                                            ),
                                        )}
                                    </ul>
                                ) : (
                                    <p className={styles.emptyText}>
                                        No additional bonuses were recorded.
                                    </p>
                                )}
                            </section>
                        </div>
                    </section>
                ) : null}
            </main>
        </>
    );
}
