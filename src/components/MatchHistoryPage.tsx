"use client";

import UiText from "@/components/UiText";

import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { API_BASE } from "@/utils/serviceUrls";
import { useAuth } from "@/contexts/AuthContext";
import { getHeroPortrait } from "@/features/heroes/heroArtwork";
import { DEFAULT_MATCH_FILTER, filterMatches, hasOverallProgress, personalResult, representedClasses, type MatchFilter, type OverallProgress } from "@/features/history/historyView";
import { formatMatchFinishedAt, formatMatchMode } from "@/utils/matchHistory";
import type { MatchHistoryListItem, MatchHistoryListResponse } from "@/types";
import styles from "@/styles/MatchHistoryPage.module.css";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";

export default function MatchHistoryPage() {
    const { language } = useGameLanguage();
    const t = (source: string) => uiText(language, source);
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
        { label: t("Matches"), value: summary.matchesPlayed },
        { label: t("Wins"), value: summary.wins },
        { label: t("Losses"), value: summary.losses },
        { label: t("Win Rate"), value: summary.winRate + "%" },
    ] : null;

    return <main className={styles.page}>
        <header className={styles.hero}><div><span className={styles.eyebrow}><UiText text="CHRONICLES OF BATTLE" /></span><h1><UiText text="Match History" /></h1><p><UiText text="Completed matches, personal outcomes, and class records." /></p></div><Link href="/mode" className={styles.backLink}><UiText text="Back to Modes ↗" /></Link></header>
        <section className={styles.panel} aria-label="Overview">
            <div className={styles.sectionHead}><div><span className={styles.eyebrow}><UiText text="OVERVIEW" /></span><h2><UiText text="Your record" /></h2></div><span className={styles.caption}><UiText text="Server confirmed totals" /></span></div>
            {progressError && <p className={styles.error} role="alert">{progressError} <button type="button" onClick={() => void loadProgress()}><UiText text="Retry" /></button></p>}
            {!summary && progressLoading ? <p className={styles.empty} role="status"><UiText text="Loading statistics…" /></p> : metrics ? <><div className={styles.metrics}>{metrics.map(metric => <div className={styles.metric} key={metric.label}><span>{metric.label}</span><strong>{metric.value}</strong></div>)}</div><details className={styles.moreStats}><summary><UiText text="More combat statistics" /></summary><div>{[["Player kills", summary!.playerKills], ["Monster kills", summary!.monsterKills], ["Deaths", summary!.deaths], ["Damage dealt", summary!.damageTotal], ["Damage taken", summary!.damageTaken], ["XP earned", summary!.totalExpGained]].filter((entry): entry is [string, number] => typeof entry[1] === "number" && Number.isFinite(entry[1])).map(([label, value]) => <span key={label}><UiText text={label} />: <strong>{value.toLocaleString()}</strong></span>)}</div></details></> : <p className={styles.empty}><UiText text="Match statistics are unavailable." /></p>}
        </section>
        <section className={styles.panel} aria-label="History and class statistics">
            <div className={styles.sectionHead}><div><span className={styles.eyebrow}><UiText text="BATTLE RECORDS" /></span><h2>{view === "matches" ? "Completed matches" : "Class statistics"}</h2></div></div>
            <div className={styles.views} role="group" aria-label="History views"><button type="button" aria-pressed={view === "matches"} onClick={() => setView("matches")}><UiText text="Matches" /></button><button type="button" aria-pressed={view === "classes"} onClick={() => setView("classes")}><UiText text="Classes" /></button></div>
            {view === "matches" ? <>
                <div className={styles.filters} aria-label="Filter loaded matches">
                    <label><UiText text="Mode" /><select value={filter.mode} onChange={event => setFilter(current => ({ ...current, mode: event.target.value as MatchFilter["mode"] }))}><option value="all"><UiText text="All modes" /></option><option value="pve"><UiText text="PvE" /></option><option value="pvp"><UiText text="PvP" /></option></select></label>
                    <label><UiText text="Class" /><select value={filter.heroClass} onChange={event => setFilter(current => ({ ...current, heroClass: event.target.value }))}><option value="all"><UiText text="All classes" /></option>{classOptions.map(heroClass => <option value={heroClass} key={heroClass}>{t(heroClass.charAt(0).toUpperCase() + heroClass.slice(1))}</option>)}</select></label>
                    <label><UiText text="Result" /><select value={filter.result} onChange={event => setFilter(current => ({ ...current, result: event.target.value as MatchFilter["result"] }))}><option value="all"><UiText text="All results" /></option><option value="winner"><UiText text="Victory" /></option><option value="survived"><UiText text="Survived" /></option><option value="eliminated"><UiText text="Eliminated" /></option></select></label>
                    <button className={styles.resetButton} type="button" onClick={() => setFilter(DEFAULT_MATCH_FILTER)}><UiText text="Reset filters" /></button>
                </div>
                <p className={styles.scopeNote}><UiText text="Filters apply to the completed matches returned by the server. Showing" /> {filtered.length} <UiText text="of" /> {items?.length ?? 0} <UiText text="loaded." /></p>
                {historyError && <p className={styles.error} role="alert">{historyError} <button type="button" onClick={() => void loadHistory()}><UiText text="Retry" /></button></p>}
                {!items && historyLoading ? <p className={styles.empty} role="status"><UiText text="Loading completed matches…" /></p> : items?.length === 0 ? <p className={styles.empty}><UiText text="No completed matches yet. Individual HEX results from an ongoing match remain in the game result screen." /></p> : filtered.length === 0 ? <p className={styles.empty}><UiText text="No loaded matches match these filters." /> <button type="button" onClick={() => setFilter(DEFAULT_MATCH_FILTER)}><UiText text="Clear filters" /></button></p> : <div className={styles.matchList}>{filtered.map(match => {
                    const result = personalResult(match.playerResultSummary);
                    return <article className={styles.matchRow} key={match.matchId}>
                        <img src={getHeroPortrait(match.characterType)} alt="" />
                        <div className={styles.matchCopy}><div className={styles.matchTitle}><h3>{formatMatchMode(match.mode)}</h3><span className={styles.resultBadge} data-tone={result.tone}>{t(result.label)}</span></div><p>{formatMatchFinishedAt(match.finishedAt, language)} · {match.characterType ? t(match.characterType.charAt(0).toUpperCase() + match.characterType.slice(1)) : t("Class unavailable")} · {match.participants?.length ?? "—"} <UiText text="participants" /></p><small><UiText text="Kills" /> {match.playerResultSummary?.kills ?? "—"}{match.damageDealt != null && <> · <UiText text="Damage" /> {match.damageDealt}</>}{match.playerRewardSummary?.expGained != null && <> · <UiText text="XP" /> +{match.playerRewardSummary.expGained}</>}</small></div>
                        <Link href={"/matches/" + encodeURIComponent(match.matchId)}><UiText text="View Details ↗" /></Link>
                    </article>;
                })}</div>}
            </> : <>
                {progressError && <p className={styles.error} role="alert">{progressError} <button type="button" onClick={() => void loadProgress()}><UiText text="Retry" /></button></p>}
                {!summary && progressLoading ? <p className={styles.empty} role="status"><UiText text="Loading class statistics…" /></p> : classes.length === 0 ? <p className={styles.empty}><UiText text="No class statistics from completed matches yet." /></p> : <div className={styles.classGrid}>{classes.map(item => <article className={styles.classCard} key={item.characterType}><div className={styles.classTop}><img src={getHeroPortrait(item.characterType)} alt="" /><div><h3>{t(item.characterType.charAt(0).toUpperCase() + item.characterType.slice(1))}</h3><span>{item.matchesPlayed} <UiText text="matches ·" /> {item.winRate}<UiText text="% win rate" /></span></div></div><div className={styles.classNumbers}>{[["Wins", item.wins], ["Losses", item.losses], ["Player kills", item.playerKills], ["Monster kills", item.monsterKills], ["Deaths", item.deaths], ["Damage", item.damageTotal]].filter((entry): entry is [string, number] => typeof entry[1] === "number" && Number.isFinite(entry[1])).map(([label, value]) => <div key={label}><span><UiText text={label} /></span><strong>{value.toLocaleString()}</strong></div>)}</div></article>)}</div>}
            </>}
        </section>
    </main>;
}
