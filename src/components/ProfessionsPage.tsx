"use client";

import React from "react";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import Link from "next/link";
import UiText from "./UiText";
import DropGuidePage from "./DropGuidePage";
import ProfessionCrafting from "./ProfessionCrafting";
import { useAuth } from "@/contexts/AuthContext";
import { API_BASE } from "@/utils/serviceUrls";
import { getHeroPortrait } from "@/features/heroes/heroArtwork";
import { ConfirmDialog } from "@/components/ui";
import styles from "@/styles/ProfessionsPage.module.css";

type Definition = { code: string; name: string; description: string; category: "gathering" | "processing" };
type Progress = Definition & { level: number; xp: number; xpToNext: number; xpRemaining: number; xpLevelStart: number; maxLevel: number; levelName: string };
type State = { guild: { built: boolean; level: number; costs: { wood: number; stone: number; iron: number; gold: number } }; characterId: number; heroClassId: string; gathering: Progress; processing: Progress | null; available: Definition[] };
type Hero = { id: string; displayName: string; characterId?: number | null; owned: boolean; enabled: boolean };

const classNames: Record<string, string> = { guardian: "Страж", berserker: "Берсерк", ranger: "Рейнджер", mystic: "Маг" };

function ProfessionLevel({ progress }: { progress: Progress }) {
    const { language } = useGameLanguage();
    const percent = progress.xpToNext > 0 ? Math.min(100, Math.max(0, (progress.xp - progress.xpLevelStart) / (progress.xpToNext - progress.xpLevelStart) * 100)) : 100;
    return <div className={styles.level}>
        <strong><UiText text="Level" /> {progress.level} / {progress.maxLevel} · <UiText text={progress.levelName} /></strong>
        <span>{progress.xpToNext > 0 ? `${progress.xp.toLocaleString(language)} / ${progress.xpToNext.toLocaleString(language)} XP` : <>{progress.xp.toLocaleString(language)} XP · <UiText text="Maximum level" /></>}</span>
        <div className={styles.progressTrack} role="progressbar" aria-label={`Опыт: ${progress.name}`} aria-valuemin={progress.xpToNext ? progress.xpLevelStart : 0} aria-valuemax={progress.xpToNext || progress.xp || 1} aria-valuenow={progress.xpToNext ? progress.xp : progress.xp || 1}><span style={{ width: `${percent}%` }} /></div>
        {progress.xpToNext > 0 && <small><UiText text="XP to next level" />: {progress.xpRemaining.toLocaleString(language)} XP</small>}
    </div>;
}

export default function ProfessionsPage() {
    const { user } = useAuth();
    const [state, setState] = React.useState<State | null>(null);
    const [heroes, setHeroes] = React.useState<Hero[]>([]);
    const [loading, setLoading] = React.useState(true);
    const [busy, setBusy] = React.useState(false);
    const [error, setError] = React.useState("");
    const [tab, setTab] = React.useState<"professions" | "gathering" | "processing">("professions");
    const busyRef = React.useRef(false);
    const [pending, setPending] = React.useState<Definition | null>(null);
    const loadSerial = React.useRef(0);

    const request = React.useCallback(async (path: string, init?: RequestInit) => {
        if (!user?.token) throw new Error("Сессия недоступна");
        const response = await fetch(`${API_BASE}${path}`, { ...init, headers: { Authorization: `Bearer ${user.token}`, ...init?.headers }, cache: "no-store" });
        const payload = await response.json().catch(() => ({}));
        if (!response.ok) throw new Error(payload.error === "guild_required" ? "Постройте Гильдию ремесленников на базе" : payload.error === "character_not_owned" ? "Этот персонаж вам не принадлежит" : payload.error === "reset_confirmation_required" ? "Подтвердите сброс прежнего ремесла" : payload.error === "profession_changed" ? "Ремесло уже изменилось. Обновите данные персонажа." : payload.error === "invalid_profession" ? "Неверная профессия" : "Не удалось загрузить профессии");
        return payload;
    }, [user?.token]);

    const load = React.useCallback(async (characterId = 0) => {
        const serial = ++loadSerial.current;
        setLoading(true);
        setError("");
        try {
            const result = await request(`/game/professions${characterId ? `?characterId=${characterId}` : ""}`) as State;
            if (serial === loadSerial.current) setState(result);
        } catch (reason) {
            if (serial === loadSerial.current) setError(reason instanceof Error ? reason.message : "Не удалось загрузить профессии");
        } finally {
            if (serial === loadSerial.current) setLoading(false);
        }
    }, [request]);

    React.useEffect(() => {
        if (!user?.token) return;
        let active = true;
        setState(null); setHeroes([]); setPending(null);
        void load();
        void request("/game/heroes").then((data: { heroes?: Hero[] }) => { if (active) setHeroes((data.heroes || []).filter(hero => hero.owned && hero.enabled && hero.characterId)); }).catch(() => {});
        return () => { active = false; ++loadSerial.current; };
    }, [user?.token, load, request]);
    React.useEffect(() => {
        const refresh = () => { if (!busyRef.current) void load(state?.characterId); };
        window.addEventListener("focus", refresh);
        return () => window.removeEventListener("focus", refresh);
    }, [load, state?.characterId]);

    const choose = async (profession: Definition, confirmReset: boolean) => {
        if (!state?.guild.built || busyRef.current || busy) return;
        busyRef.current = true;
        const characterId = state.characterId;
        const serial = ++loadSerial.current;
        setBusy(true);
        setError("");
        try {
            const next = await request("/game/professions/choose", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ characterId: state.characterId, code: profession.code, expectedCode: state.processing?.code || "", confirmReset }) }) as State;
            if (serial === loadSerial.current && next.characterId === characterId) { setState(next); setPending(null); }
        } catch (reason) {
            if (serial === loadSerial.current) setError(reason instanceof Error ? reason.message : "Не удалось выбрать ремесло");
        } finally {
            busyRef.current = false;
            setBusy(false);
        }
    };

    return <main className={styles.page}>
        <div className={styles.container}>
            <header className={styles.header}><span className={styles.eyebrow}>ПУТЬ МАСТЕРА</span><h1><UiText text="Artisans' Guild" /></h1><p>У каждого героя своя добывающая профессия и одно выбранное ремесло.</p></header>
            <div className={styles.heroPicker}>
                <label htmlFor="profession-hero">Персонаж</label>
                <select id="profession-hero" value={state?.characterId || ""} disabled={loading || busy} onChange={event => { setPending(null); void load(Number(event.target.value)); }}>
                    {!state && <option value="">Загрузка…</option>}
                    {state && !heroes.some(hero => hero.characterId === state.characterId) && <option value={state.characterId}>{classNames[state.heroClassId] || state.heroClassId}</option>}
                    {heroes.map(hero => <option key={hero.characterId} value={hero.characterId || ""}>{hero.displayName}</option>)}
                </select>
                {state && <span>{classNames[state.heroClassId] || state.heroClassId}</span>}
            </div>
            {error && <p className={styles.error} role="alert">{error} <button type="button" onClick={() => void load(state?.characterId)}>Повторить загрузку</button></p>}
            {loading && <p className={styles.loading} role="status">Загрузка профессий…</p>}
            {state && !state.guild.built && <section className={styles.panel}><h2><UiText text="Guild required" /></h2><p><UiText text="Build the Artisans' Guild on your base to unlock professions for all your heroes." /></p><p><UiText text="Construction cost" />: {state.guild.costs.wood} <UiText text="Wood" /> · {state.guild.costs.stone} <UiText text="Stone" /> · {state.guild.costs.iron} <UiText text="Iron" /> · {state.guild.costs.gold} <UiText text="Gold" /></p><Link href="/base?building=guild" className={styles.guildLink}><UiText text="Open Base" /></Link></section>}
            {state?.guild.built && <>
            <div className={styles.tabs} role="group" aria-label="Guild sections">{([['professions', 'My professions'], ['gathering', 'Gathering'], ['processing', 'Processing']] as const).map(([id, label]) => <button key={id} type="button" aria-pressed={tab === id} disabled={busy} onClick={() => setTab(id)}><UiText text={label} /></button>)}</div>
            {tab === "gathering" && <section><h2><UiText text={state.gathering.name} /></h2><ProfessionLevel progress={state.gathering} /><p><UiText text="Two XP sources: combat trophies and peaceful gathering. Progress is saved only after successful extraction." /></p><DropGuidePage embedded initialClass={state.heroClassId} professionLevel={state.gathering.level} /></section>}
            {tab === "processing" && <section className={styles.panel}><h2><UiText text={state.processing?.name || "Ремесло не выбрано"} /></h2>{state.processing && <p><UiText text={state.processing.description} /></p>}<div>{state.processing ? <ProfessionCrafting key={`${state.characterId}:${state.processing.code}`} characterId={state.characterId} profession={state.processing.code} currentLevel={state.processing.level} maxLevel={state.processing.maxLevel} onBusyChange={next => { busyRef.current = next; setBusy(next); }} onCrafted={() => load(state.characterId)} /> : <p><UiText text="Choose a processing profession to craft items." /></p>}</div><button type="button" className={styles.guildLink} disabled={busy} onClick={() => setTab("professions")}><UiText text="My professions" /></button></section>}
            {tab === "professions" && <div className={styles.columns}>
                <section className={styles.panel} aria-label="Добывающая профессия">
                    <span className={styles.eyebrow}>ДОБЫЧА · ПО КЛАССУ</span>
                    <div className={styles.professionHeading}><img src={getHeroPortrait(state.heroClassId)} alt="" /><div><h2><UiText text={state.gathering.name} /></h2><p><UiText text={state.gathering.description} /></p></div></div>
                    <ProfessionLevel progress={state.gathering} />
                    <div className={styles.directions}><h3><UiText text="Gathering sources" /></h3><p><UiText text="Combat profession materials" /></p><p><UiText text="Peaceful professional resources" /></p><small><UiText text="Two XP sources: combat trophies and peaceful gathering. Progress is saved only after successful extraction." /></small><p><UiText text="Gather on the object's HEX cell for 0.5 AP. The Guild must be built before the match." /></p></div>
                </section>
                <section className={styles.panel} aria-label="Перерабатывающая профессия">
                    <span className={styles.eyebrow}>РЕМЕСЛО · НА ВЫБОР</span>
                    {state.processing ? <div className={styles.activeCraft}><h2><UiText text={state.processing.name} /></h2><p><UiText text={state.processing.description} /></p><ProfessionLevel progress={state.processing} /></div> : <div className={styles.activeCraft}><h2>Ремесло не выбрано</h2><p>Первый выбор бесплатный. Каждому персонажу доступно любое из пяти ремёсел.</p></div>}
                    <h3>{state.processing ? "Сменить ремесло" : "Выбрать ремесло"}</h3>
                    <div className={styles.choices}>{state.available.map(profession => <article key={profession.code} className={styles.choice} data-active={profession.code === state.processing?.code}><div><strong><UiText text={profession.name} /></strong><p><UiText text={profession.description} /></p></div><button type="button" disabled={busy || loading || profession.code === state.processing?.code} onClick={() => state.processing ? setPending(profession) : void choose(profession, false)}>{profession.code === state.processing?.code ? "Выбрано" : state.processing ? "Сменить" : "Выбрать"}</button></article>)}</div>
                </section>
            </div>}</>}
        </div>
        <ConfirmDialog open={!!pending} title="Сменить ремесло?" description={`Уровень и XP ремесла «${state?.processing?.name || ""}» будут безвозвратно сброшены. «${pending?.name || ""}» начнётся с уровня 1 и 0 XP. Имущество и добывающая профессия сохранятся.`} confirmLabel="Сбросить и сменить" cancelLabel="Отмена" tone="danger" busy={busy} onConfirm={() => pending && void choose(pending, true)} onCancel={() => { if (!busy) setPending(null); }} />
    </main>;
}
