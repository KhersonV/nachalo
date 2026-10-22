"use client";

import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { API_BASE } from "@/utils/serviceUrls";
import { useAuth } from "@/contexts/AuthContext";
import { getHeroPortrait } from "@/features/heroes/heroArtwork";
import { DEFAULT_MATCH_FILTER, filterMatches, hasOverallProgress, personalResult, representedClasses, type MatchFilter, type OverallProgress } from "@/features/history/historyView";
import { formatMatchFinishedAt, formatMatchMode } from "@/utils/matchHistory";
import type { MatchHistoryListItem, MatchHistoryListResponse } from "@/types";
import styles from "@/styles/MatchHistoryPage.module.css";

export default function MatchHistoryPage() {
    const { user } = useAuth();
    const token = user?.token ?? "";
    const [history, setHistory] = useState<{ token: string; items: MatchHistoryListItem[] } | null>(null);
    const [progress, setProgress] = useState<{ token: string; data: OverallProgress | null } | null>(null);
    const [historyError, setHistoryError] = useState("");
    const [progressError, setProgressError] = useState("");
    const [historyLoading, setHistoryLoading] = useState(true);
    const [progressLoading, setProgressLoading] = useState(true);
    const [view, setView] = useState<"matches" | "classes">("matches");
    const [filter, setFilter] = useState<MatchFilter>(DEFAULT_MATCH_FILTER);
    const historyRequest = useRef(0);
    const progressRequest = useRef(0);

    const loadHistory = useCallback(async () => {
        if (!token) return;
        const requestId = ++historyRequest.current;
        setHistoryLoading(true); setHistoryError("");
        try {
            const response = await fetch(API_BASE + "/game/matches/history", { headers: { Authorization: "Bearer " + token }, cache: "no-store" });
            if (!response.ok) throw new Error("Failed to load completed matches");
            const payload = await response.json() as MatchHistoryListResponse;
            if (!Array.isArray(payload.data)) throw new Error("History response is incomplete");
            if (requestId === historyRequest.current) setHistory({ token, items: payload.data });
        } catch (error) {
            if (requestId === historyRequest.current) setHistoryError(error instanceof Error ? error.message : "History unavailable");
        } finally { if (requestId === historyRequest.current) setHistoryLoading(false); }
    }, [token]);

    const loadProgress = useCallback(async () => {
        if (!token) return;
        const requestId = ++progressRequest.current;
        setProgressLoading(true); setProgressError("");
        try {
            const response = await fetch(API_BASE + "/game/profile", { headers: { Authorization: "Bearer " + token }, cache: "no-store" });
            if (!response.ok) throw new Error("Failed to load player statistics");
            const payload = await response.json() as { data?: { progress?: OverallProgress | null } };
            if (requestId === progressRequest.current) setProgress({ token, data: payload.data?.progress ?? null });
        } catch (error) {
            if (requestId === progressRequest.current) setProgressError(error instanceof Error ? error.message : "Statistics unavailable");
        } finally { if (requestId === progressRequest.current) setProgressLoading(false); }
    }, [token]);

    useEffect(() => {
        void loadHistory();
        void loadProgress();
        return () => { ++historyRequest.current; ++progressRequest.current; };
    }, [loadHistory, loadProgress]);

    const items = history?.token === token ? history.items : null;
    const summary = progress?.token === token ? progress.data : null;
    const classes = representedClasses(summary);
    const filtered = useMemo(() => items ? filterMatches(items, filter) : [], [items, filter]);
    const classOptions = useMemo(() => Array.from(new Set((items ?? []).map(item => item.characterType?.toLowerCase()).filter((value): value is string => Boolean(value)))).sort(), [items]);
    const metrics = hasOverallProgress(summary) ? [
        { label: "Matches", value: summary.matchesPlayed },
        { label: "Wins", value: summary.wins },
        { label: "Losses", value: summary.losses },
        { label: "Win Rate", value: summary.winRate + "%" },
    ] : null;

    return <main className={styles.page}>
        <header className={styles.hero}><div><span className={styles.eyebrow}>CHRONICLES OF BATTLE</span><h1>Match History</h1><p>Completed matches, personal outcomes, and class records.</p></div><Link href="/mode" className={styles.backLink}>Back to Modes ↗</Link></header>
        <section className={styles.panel} aria-label="Overview">
            <div className={styles.sectionHead}><div><span className={styles.eyebrow}>OVERVIEW</span><h2>Your record</h2></div><span className={styles.caption}>Server confirmed totals</span></div>
            {progressError && <p className={styles.error} role="alert">{progressError} <button type="button" onClick={() => void loadProgress()}>Retry</button></p>}
            {!summary && progressLoading ? <p className={styles.empty} role="status">Loading statistics…</p> : metrics ? <><div className={styles.metrics}>{metrics.map(metric => <div className={styles.metric} key={metric.label}><span>{metric.label}</span><strong>{metric.value}</strong></div>)}</div><details className={styles.moreStats}><summary>More combat statistics</summary><div>{[["Player kills", summary!.playerKills], ["Monster kills", summary!.monsterKills], ["Deaths", summary!.deaths], ["Damage dealt", summary!.damageTotal], ["Damage taken", summary!.damageTaken], ["XP earned", summary!.totalExpGained]].filter((entry): entry is [string, number] => typeof entry[1] === "number" && Number.isFinite(entry[1])).map(([label, value]) => <span key={label}>{label}: <strong>{value.toLocaleString()}</strong></span>)}</div></details></> : <p className={styles.empty}>Match statistics are unavailable.</p>}
        </section>
        <section className={styles.panel} aria-label="History and class statistics">
            <div className={styles.sectionHead}><div><span className={styles.eyebrow}>BATTLE RECORDS</span><h2>{view === "matches" ? "Completed matches" : "Class statistics"}</h2></div></div>
            <div className={styles.views} role="group" aria-label="History views"><button type="button" aria-pressed={view === "matches"} onClick={() => setView("matches")}>Matches</button><button type="button" aria-pressed={view === "classes"} onClick={() => setView("classes")}>Classes</button></div>
            {view === "matches" ? <>
                <div className={styles.filters} aria-label="Filter loaded matches">
                    <label>Mode<select value={filter.mode} onChange={event => setFilter(current => ({ ...current, mode: event.target.value as MatchFilter["mode"] }))}><option value="all">All modes</option><option value="pve">PvE</option><option value="pvp">PvP</option></select></label>
                    <label>Class<select value={filter.heroClass} onChange={event => setFilter(current => ({ ...current, heroClass: event.target.value }))}><option value="all">All classes</option>{classOptions.map(heroClass => <option value={heroClass} key={heroClass}>{heroClass}</option>)}</select></label>
                    <label>Result<select value={filter.result} onChange={event => setFilter(current => ({ ...current, result: event.target.value as MatchFilter["result"] }))}><option value="all">All results</option><option value="winner">Victory</option><option value="survived">Survived</option><option value="eliminated">Eliminated</option></select></label>
                    <button className={styles.resetButton} type="button" onClick={() => setFilter(DEFAULT_MATCH_FILTER)}>Reset filters</button>
                </div>
                <p className={styles.scopeNote}>Filters apply to the completed matches returned by the server. Showing {filtered.length} of {items?.length ?? 0} loaded.</p>
                {historyError && <p className={styles.error} role="alert">{historyError} <button type="button" onClick={() => void loadHistory()}>Retry</button></p>}
                {!items && historyLoading ? <p className={styles.empty} role="status">Loading completed matches…</p> : items?.length === 0 ? <p className={styles.empty}>No completed matches yet. Individual HEX results from an ongoing match remain in the game result screen.</p> : filtered.length === 0 ? <p className={styles.empty}>No loaded matches match these filters. <button type="button" onClick={() => setFilter(DEFAULT_MATCH_FILTER)}>Clear filters</button></p> : <div className={styles.matchList}>{filtered.map(match => {
                    const result = personalResult(match.playerResultSummary);
                    return <article className={styles.matchRow} key={match.matchId}>
                        <img src={getHeroPortrait(match.characterType)} alt="" />
                        <div className={styles.matchCopy}><div className={styles.matchTitle}><h3>{formatMatchMode(match.mode)}</h3><span className={styles.resultBadge} data-tone={result.tone}>{result.label}</span></div><p>{formatMatchFinishedAt(match.finishedAt)} · {match.characterType || "Class unavailable"} · {match.participants?.length ?? "—"} participants</p><small>{match.playerResultSummary?.kills != null ? "Kills " + match.playerResultSummary.kills : "Kills —"}{match.damageDealt != null ? " · Damage " + match.damageDealt : ""}{match.playerRewardSummary?.expGained != null ? " · XP +" + match.playerRewardSummary.expGained : ""}</small></div>
                        <Link href={"/matches/" + encodeURIComponent(match.matchId)}>View Details ↗</Link>
                    </article>;
                })}</div>}
            </> : <>
                {progressError && <p className={styles.error} role="alert">{progressError} <button type="button" onClick={() => void loadProgress()}>Retry</button></p>}
                {!summary && progressLoading ? <p className={styles.empty} role="status">Loading class statistics…</p> : classes.length === 0 ? <p className={styles.empty}>No class statistics from completed matches yet.</p> : <div className={styles.classGrid}>{classes.map(item => <article className={styles.classCard} key={item.characterType}><div className={styles.classTop}><img src={getHeroPortrait(item.characterType)} alt="" /><div><h3>{item.characterType}</h3><span>{item.matchesPlayed} matches · {item.winRate}% win rate</span></div></div><div className={styles.classNumbers}>{[["Wins", item.wins], ["Losses", item.losses], ["Player kills", item.playerKills], ["Monster kills", item.monsterKills], ["Deaths", item.deaths], ["Damage", item.damageTotal]].filter((entry): entry is [string, number] => typeof entry[1] === "number" && Number.isFinite(entry[1])).map(([label, value]) => <div key={label}><span>{label}</span><strong>{value.toLocaleString()}</strong></div>)}</div></article>)}</div>}
            </>}
        </section>
    </main>;
}
