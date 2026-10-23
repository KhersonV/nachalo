"use client";

import React from "react";
import { useAuth } from "@/contexts/AuthContext";
import { API_BASE } from "@/utils/serviceUrls";
import { getHeroPortrait } from "@/features/heroes/heroArtwork";
import { ConfirmDialog } from "@/components/ui";
import styles from "@/styles/ProfessionsPage.module.css";

type Definition = { code: string; name: string; description: string; category: "gathering" | "processing" };
type Progress = Definition & { level: number; xp: number; xpToNext: number; xpRemaining: number };
type State = { characterId: number; heroClassId: string; gathering: Progress; processing: Progress | null; available: Definition[] };
type Hero = { id: string; displayName: string; characterId?: number | null; owned: boolean; enabled: boolean };

const classNames: Record<string, string> = { guardian: "Страж", berserker: "Берсерк", ranger: "Рейнджер", mystic: "Маг" };

function ProfessionLevel({ progress }: { progress: Progress }) {
    const percent = progress.xpToNext > 0 ? Math.min(100, Math.max(0, progress.xp / progress.xpToNext * 100)) : 100;
    return <div className={styles.level}>
        <strong>Уровень {progress.level} / 100</strong>
        <span>{progress.xpToNext > 0 ? `${progress.xp.toLocaleString("ru-RU")} / ${progress.xpToNext.toLocaleString("ru-RU")} XP` : "Максимальный уровень"}</span>
        <div className={styles.progressTrack} role="progressbar" aria-label={`Опыт: ${progress.name}`} aria-valuemin={0} aria-valuemax={progress.xpToNext || 1} aria-valuenow={progress.xpToNext ? progress.xp : 1}><span style={{ width: `${percent}%` }} /></div>
        {progress.xpToNext > 0 && <small>До следующего уровня: {progress.xpRemaining.toLocaleString("ru-RU")} XP</small>}
    </div>;
}

export default function ProfessionsPage() {
    const { user } = useAuth();
    const [state, setState] = React.useState<State | null>(null);
    const [heroes, setHeroes] = React.useState<Hero[]>([]);
    const [loading, setLoading] = React.useState(true);
    const [busy, setBusy] = React.useState(false);
    const [error, setError] = React.useState("");
    const [pending, setPending] = React.useState<Definition | null>(null);
    const loadSerial = React.useRef(0);

    const request = React.useCallback(async (path: string, init?: RequestInit) => {
        if (!user?.token) throw new Error("Сессия недоступна");
        const response = await fetch(`${API_BASE}${path}`, { ...init, headers: { Authorization: `Bearer ${user.token}`, ...init?.headers }, cache: "no-store" });
        const payload = await response.json().catch(() => ({}));
        if (!response.ok) throw new Error(payload.error === "character_not_owned" ? "Этот персонаж вам не принадлежит" : payload.error === "reset_confirmation_required" ? "Подтвердите сброс прежнего ремесла" : payload.error === "profession_changed" ? "Ремесло уже изменилось. Обновите данные персонажа." : payload.error === "invalid_profession" ? "Неверная профессия" : "Не удалось загрузить профессии");
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
        void load();
        void request("/game/heroes").then((data: { heroes?: Hero[] }) => setHeroes((data.heroes || []).filter(hero => hero.owned && hero.enabled && hero.characterId))).catch(() => {});
    }, [user?.token, load, request]);

    const choose = async (profession: Definition, confirmReset: boolean) => {
        if (!state || busy) return;
        setBusy(true);
        setError("");
        try {
            const next = await request("/game/professions/choose", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ characterId: state.characterId, code: profession.code, expectedCode: state.processing?.code || "", confirmReset }) }) as State;
            setState(next);
            setPending(null);
        } catch (reason) {
            setError(reason instanceof Error ? reason.message : "Не удалось выбрать ремесло");
        } finally {
            setBusy(false);
        }
    };

    return <main className={styles.page}>
        <div className={styles.container}>
            <header className={styles.header}><span className={styles.eyebrow}>ПУТЬ МАСТЕРА</span><h1>Профессии</h1><p>У каждого героя своя добывающая профессия и одно выбранное ремесло.</p></header>
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
            {state && <div className={styles.columns}>
                <section className={styles.panel} aria-label="Добывающая профессия">
                    <span className={styles.eyebrow}>ДОБЫЧА · ПО КЛАССУ</span>
                    <div className={styles.professionHeading}><img src={getHeroPortrait(state.heroClassId)} alt="" /><div><h2>{state.gathering.name}</h2><p>{state.gathering.description}</p></div></div>
                    <ProfessionLevel progress={state.gathering} />
                    <div className={styles.directions}><h3>Будущие направления</h3><p>Материалы из монстров</p><p>Ресурсы с HEX-клеток</p><small>Добыча и профессиональный XP появятся на следующем этапе. Оба направления будут повышать эту профессию.</small></div>
                </section>
                <section className={styles.panel} aria-label="Перерабатывающая профессия">
                    <span className={styles.eyebrow}>РЕМЕСЛО · НА ВЫБОР</span>
                    {state.processing ? <div className={styles.activeCraft}><h2>{state.processing.name}</h2><p>{state.processing.description}</p><ProfessionLevel progress={state.processing} /></div> : <div className={styles.activeCraft}><h2>Ремесло не выбрано</h2><p>Первый выбор бесплатный. Каждому персонажу доступно любое из пяти ремёсел.</p></div>}
                    <h3>{state.processing ? "Сменить ремесло" : "Выбрать ремесло"}</h3>
                    <div className={styles.choices}>{state.available.map(profession => <article key={profession.code} className={styles.choice} data-active={profession.code === state.processing?.code}><div><strong>{profession.name}</strong><p>{profession.description}</p></div><button type="button" disabled={busy || loading || profession.code === state.processing?.code} onClick={() => state.processing ? setPending(profession) : void choose(profession, false)}>{profession.code === state.processing?.code ? "Выбрано" : state.processing ? "Сменить" : "Выбрать"}</button></article>)}</div>
                </section>
            </div>}
        </div>
        <ConfirmDialog open={!!pending} title="Сменить ремесло?" description={`Уровень и XP ремесла «${state?.processing?.name || ""}» будут безвозвратно сброшены. «${pending?.name || ""}» начнётся с уровня 1 и 0 XP. Имущество и добывающая профессия сохранятся.`} confirmLabel="Сбросить и сменить" cancelLabel="Отмена" tone="danger" busy={busy} onConfirm={() => pending && void choose(pending, true)} onCancel={() => { if (!busy) setPending(null); }} />
    </main>;
}
