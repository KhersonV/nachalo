"use client";

import UiText from "@/components/UiText";

import React, { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { API_BASE } from "@/utils/serviceUrls";
import { useAuth } from "@/contexts/AuthContext";
import { getHeroPortrait } from "@/features/heroes/heroArtwork";
import { personalResult } from "@/features/history/historyView";
import { formatMatchFinishedAt, formatMatchMode, formatWinnerSummary, rewardLabel } from "@/utils/matchHistory";
import type { MatchHistoryDetailItem, MatchHistoryDetailResponse } from "@/types";
import styles from "@/styles/MatchHistoryDetailsPage.module.css";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";

function resolveErrorMessage(code: string): string {
    switch (code) {
        case "match_not_completed": return "This match is still in progress. History becomes available after the match ends; your individual HEX result remains in the game result screen.";
        case "match_access_denied": return "You can only view statistics for matches where you participated.";
        case "match_not_found": return "Match statistics were not found.";
        default: return "Failed to load match statistics.";
    }
}

export default function MatchHistoryDetailsPage({ instanceId }: { instanceId: string }) {
    const { language } = useGameLanguage();
    const t = (source: string) => uiText(language, source);
    const { user } = useAuth();
    const token = user?.token ?? "";
    const [record, setRecord] = useState<{ key: string; data: MatchHistoryDetailItem } | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");
    const request = useRef(0);
    const key = token + ":" + instanceId;

    const load = useCallback(async () => {
        if (!token || !instanceId) return;
        const requestId = ++request.current;
        setLoading(true); setError("");
        try {
            const response = await fetch(API_BASE + "/game/matches/history/" + encodeURIComponent(instanceId), {
                headers: { Authorization: "Bearer " + token }, cache: "no-store",
            });
            if (!response.ok) {
                let code = "";
                try { code = (await response.json() as { error?: string }).error ?? ""; } catch { /* Keep generic error. */ }
                throw new Error(resolveErrorMessage(code));
            }
            const payload = await response.json() as MatchHistoryDetailResponse;
            if (!payload.data || payload.data.matchId !== instanceId) throw new Error("The server returned incomplete match details.");
            if (requestId === request.current) setRecord({ key, data: payload.data });
        } catch (reason) {
            if (requestId === request.current) setError(reason instanceof Error ? reason.message : "Failed to load match statistics.");
        } finally { if (requestId === request.current) setLoading(false); }
    }, [token, instanceId, key]);

    useEffect(() => { void load(); return () => { ++request.current; }; }, [load]);

    const details = record?.key === key ? record.data : null;
    const result = personalResult(details?.playerResultSummary);
    const stats = details?.currentPlayerStats;
    const personalMetrics = stats ? [
        ["Player kills", stats.playerKills], ["Monster kills", stats.monsterKills], ["Deaths", stats.deaths],
        ["Damage dealt", stats.damageDealt], ["Damage taken", stats.damageTaken], ["Damage to players", stats.damageToPlayers],
        ["Damage to monsters", stats.damageToMonsters], ["Placement", stats.placement > 0 ? "#" + stats.placement : "—"],
    ] : [];

    return <main className={styles.page}>
        <div className={styles.topRow}><div><span className={styles.eyebrow}><UiText text="BATTLE RECORD" /></span><h1><UiText text="Match Details" /></h1></div><Link href="/matches" className={styles.backLink}><UiText text="Back to History ↗" /></Link></div>
        {!details && loading ? <section className={styles.stateCard} role="status"><UiText text="Loading match statistics…" /></section> : null}
        {error && <section className={styles.stateCard} role="alert">{t(error)} <button type="button" onClick={() => void load()}><UiText text="Retry" /></button></section>}
        {details && <>
            <section className={styles.summary} aria-label="Match summary">
                <img className={styles.heroPortrait} src={getHeroPortrait(details.characterType)} alt="" />
                <div className={styles.summaryCopy}><span className={styles.eyebrow}><UiText text="FINAL MATCH RECORD" /></span><h2>{formatMatchMode(details.mode)}</h2><p>{formatMatchFinishedAt(details.finishedAt, language)} · {details.characterType ? t(details.characterType.charAt(0).toUpperCase() + details.characterType.slice(1)) : t("Class unavailable")}</p><small><UiText text="Match ID:" /> {details.matchId}</small></div>
                <div className={styles.summaryResult}><span className={styles.resultBadge} data-tone={result.tone}>{t(result.label)}</span><p><UiText text="Personal result" /></p>{details.playerResultSummary.placement > 0 && <small><UiText text="Placement #" />{details.playerResultSummary.placement}</small>}</div>
            </section>
            <div className={styles.contentGrid}>
                <section className={styles.panel} aria-label="Personal performance"><div className={styles.sectionHead}><span className={styles.eyebrow}><UiText text="PERSONAL PERFORMANCE" /></span><h2><UiText text="Your match" /></h2></div><div className={styles.metrics}>{personalMetrics.filter((entry): entry is [string, number | string] => entry[1] !== undefined && entry[1] !== null).map(([label, value]) => <div key={label}><span>{t(label)}</span><strong>{typeof value === "number" ? value.toLocaleString() : value}</strong></div>)}</div></section>
                <section className={styles.panel} aria-label="Rewards"><div className={styles.sectionHead}><span className={styles.eyebrow}><UiText text="SERVER RECORD" /></span><h2><UiText text="Rewards" /></h2></div><div className={styles.rewardTotals}><div><span><UiText text="XP gained" /></span><strong>{stats?.expGained ?? "—"}</strong></div><div><span><UiText text="Currency gained" /></span><strong>{stats?.currencyGained ?? "—"}</strong></div></div>{stats?.rewards?.length ? <ul className={styles.rewardList}>{stats.rewards.map((reward, index) => <li key={reward.type + ":" + index}><span>{t(rewardLabel(reward.type))}</span><strong>{reward.amount}</strong></li>)}</ul> : <p className={styles.empty}><UiText text="No rewards recorded for this player." /></p>}
                    {stats?.otherBonuses?.length ? <div className={styles.bonuses}><h3><UiText text="Recorded bonuses" /></h3>{stats.otherBonuses.map(bonus => <p key={bonus.type}>{bonus.label}{bonus.amount !== undefined ? " · " + bonus.amount + " currency" : ""}{bonus.expBonus !== undefined ? " · " + bonus.expBonus + " XP" : ""}</p>)}</div> : null}
                </section>
                <section className={styles.panel} aria-label="Participants"><div className={styles.sectionHead}><span className={styles.eyebrow}><UiText text="MATCH ROSTER" /></span><h2><UiText text="Participants" /></h2></div><p className={styles.winnerNote}>{formatWinnerSummary(details.winner, details.participants ?? [], language)}</p>{details.participants?.length ? <div className={styles.participants}>{details.participants.map(player => <div className={styles.participant} key={player.userId}><span><strong>{player.name}</strong>{player.characterType && <small>{t(player.characterType.charAt(0).toUpperCase() + player.characterType.slice(1))}</small>}</span><span>{player.groupId > 0 ? t("Team") + " " + player.groupId : t("Solo")}{player.placement && player.placement > 0 ? " · #" + player.placement : ""}</span><span>{t(player.isWinner ? "Winner" : player.survived ? "Survived" : "Eliminated")}</span></div>)}</div> : <p className={styles.empty}><UiText text="Participant details unavailable." /></p>}</section>
                <section className={styles.panel} aria-label="Recorded artifacts"><div className={styles.sectionHead}><span className={styles.eyebrow}><UiText text="FINAL SNAPSHOT" /></span><h2><UiText text="Artifacts recorded" /></h2></div>{stats?.artifacts?.length ? <div className={styles.artifacts}>{stats.artifacts.map(artifact => <div key={artifact.itemId + ":" + artifact.name}><strong>{t(artifact.name)}</strong><span>×{artifact.count}</span>{artifact.description && <small>{t(artifact.description)}</small>}</div>)}</div> : <p className={styles.empty}><UiText text="No artifacts recorded in the final snapshot." /></p>}</section>
            </div>
        </>}
    </main>;
}
