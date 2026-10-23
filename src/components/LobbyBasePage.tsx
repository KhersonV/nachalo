"use client";

import UiText from "@/components/UiText";

import React from "react";
import Link from "next/link";
import { API_BASE } from "@/utils/serviceUrls";
import { useAuth } from "../contexts/AuthContext";
import { getHeroPortrait } from "@/features/heroes/heroArtwork";
import { resourceAsset } from "@/utils/resourceAssets";
import { BUILDING_KEYS, RESOURCE_KEYS, buildActionAllowed, buildingStatus, confirmedHeroAction, heroAction, resourceMissing, selectedBuilding, type BuildingKey, type ResourceKey } from "@/features/base/baseView";
import type { BaseState, BuildingState, HeroesState, HeroState } from "../types";
import { ConfirmDialog, EmptyState, ErrorState, LoadingState, StatusMessage } from "./ui";
import StorageLoadout from "./StorageLoadout";
import styles from "../styles/LobbyBasePage.module.css";

const BUILDINGS: Record<BuildingKey, { name: string; description: string; path: string }> = {
    forge: { name: "Forge", description: "Blueprints and tools for the road.", path: "/game/base/forge/build" },
    library: { name: "Library", description: "Knowledge kept within the stronghold.", path: "/game/base/library/build" },
    tavern: { name: "Tavern", description: "Recruit and ready your heroes.", path: "/game/base/tavern/build" },
};
const RESOURCES: Record<ResourceKey, { name: string; image: string }> = {
    wood: { name: "Wood", image: resourceAsset("wood")! },
    stone: { name: "Stone", image: resourceAsset("stone")! },
    iron: { name: "Iron", image: resourceAsset("iron")! },
};
const STATUS_LABELS = { built: "Built", ready: "Ready to build", missing: "Resources needed", unavailable: "Unavailable" };

function readBaseState(value: unknown): BaseState {
    if (!value || typeof value !== "object") throw new Error("Base state is unavailable");
    const raw = value as Record<string, unknown>;
    for (const key of BUILDING_KEYS) {
        const building = raw[key] as Partial<BuildingState> | undefined;
        if (!building || typeof building.level !== "number" || typeof building.built !== "boolean" || typeof building.canBuild !== "boolean" || !building.costs || !building.resources || !Array.isArray(building.unlockables)) {
            throw new Error("Base state is incomplete");
        }
    }
    return value as BaseState;
}

function readHeroesState(value: unknown): HeroesState {
    if (!value || typeof value !== "object" || !Array.isArray((value as HeroesState).heroes)) throw new Error("Hero roster is unavailable");
    const state = value as HeroesState;
    return { ...state, heroes: [...state.heroes].filter(hero => hero && typeof hero.id === "string" && hero.enabled).sort((a, b) => a.sortOrder - b.sortOrder) };
}

function requestError(text: string, fallback: string): string {
    const messages: Record<string, string> = {
        not_enough_resources: "Not enough resources for this building.",
        forge_already_built: "Forge is already built.",
        library_already_built: "Library is already built.",
        tavern_already_built: "Tavern is already built.",
        hero_already_owned: "This hero is already owned.",
        not_enough_gold: "Not enough gold to hire this hero.",
        tavern_required: "Build the Tavern before hiring heroes.",
        hero_not_owned: "You do not own this hero yet.",
        hero_not_found: "Hero not found.",
    };
    const code = Object.keys(messages).find(key => text.includes(key));
    return code ? messages[code] : fallback;
}

function ResourceSummary({ state, gold }: { state: BaseState | null; gold: number | null }) {
    const resources = state?.forge.resources;
    return <div className={styles.resourceSummary} aria-label="Base resources">
        <div className={styles.resourcePill}><img src={resourceAsset("gold")!} alt="" /><span><UiText text="Gold" /></span><strong>{gold === null ? "—" : gold.toLocaleString()}</strong></div>
        {RESOURCE_KEYS.map(key => <div className={styles.resourcePill} key={key}><img src={RESOURCES[key].image} alt="" /><span><UiText text={RESOURCES[key].name} /></span><strong>{resources ? resources[key].toLocaleString() : "—"}</strong></div>)}
    </div>;
}

function BuildingSelector({ state, selected, onSelect }: { state: BaseState; selected: BuildingKey; onSelect: (key: BuildingKey) => void }) {
    return <div className={styles.buildingSelector} role="group" aria-label="Choose a building">
        {BUILDING_KEYS.map(key => {
            const building = selectedBuilding(state, key);
            const status = buildingStatus(building);
            return <button key={key} type="button" className={`${styles.buildingCard} ${styles[key]} ${selected === key ? styles.buildingCardSelected : ""}`} aria-pressed={selected === key} onClick={() => onSelect(key)}>
                <span className={styles.buildingTopline}><span><UiText text="STRONGHOLD" /></span><span><UiText text="LEVEL" /> {building.level}</span></span>
                <span className={styles.buildingCardCopy}><strong><UiText text={BUILDINGS[key].name} /></strong><small><UiText text={STATUS_LABELS[status]} /></small></span>
            </button>;
        })}
    </div>;
}

function BuildingCost({ building }: { building: BuildingState }) {
    return <div className={styles.costGrid} aria-label="Building cost">
        {RESOURCE_KEYS.map(key => {
            const missing = resourceMissing(building, key);
            return <div key={key} className={`${styles.costItem} ${missing > 0 ? styles.costMissing : ""}`}>
                <img src={RESOURCES[key].image} alt="" />
                <span><UiText text={RESOURCES[key].name} /><small><UiText text="Have" /> {building.resources[key]}</small></span>
                <strong>{building.costs[key]}{missing > 0 && <small><UiText text="Need" /> {missing}</small>}</strong>
            </div>;
        })}
    </div>;
}

function TavernHeroCard({ hero, busy, onHire, onActivate }: { hero: HeroState; busy: boolean; onHire: (hero: HeroState) => void; onActivate: (hero: HeroState) => void }) {
    const action = heroAction(hero);
    const reason = hero.lockReason === "tavern_required" ? "Tavern required" : hero.lockReason === "not_enough_gold" ? "Not enough gold" : hero.lockReason ? hero.lockReason.replaceAll("_", " ") : "Unavailable";
    return <article className={`${styles.heroCard} ${hero.active ? styles.heroCardActive : ""}`}>
        <img className={styles.heroPortrait} src={getHeroPortrait(hero.id, hero.image)} alt="" />
        <div className={styles.heroText}><span className={styles.heroStatus}><UiText text={hero.active ? "ACTIVE" : hero.owned ? "OWNED" : hero.canHire ? "AVAILABLE" : "LOCKED"} /></span><strong>{hero.displayName}</strong><small><UiText text={hero.description} /></small>{!hero.owned && <span className={styles.hirePrice}>{hero.unlockPrice.toLocaleString()} <UiText text="Gold" /></span>}</div>
        {action === "active" ? <span className={styles.activeTag}><UiText text="Selected for matches" /></span> : action === "locked" ? <span className={styles.lockReason}><UiText text={reason} /></span> : <button type="button" className={styles.heroAction} disabled={busy} onClick={() => action === "hire" ? onHire(hero) : onActivate(hero)}><UiText text={busy ? "Working…" : action === "hire" ? "Hire" : "Set Active"} /></button>}
    </article>;
}

export default function LobbyBasePage() {
    const { user } = useAuth();
    const [baseState, setBaseState] = React.useState<BaseState | null>(null);
    const [heroesState, setHeroesState] = React.useState<HeroesState | null>(null);
    const [selected, setSelected] = React.useState<BuildingKey>("forge");
    const [confirmBuild, setConfirmBuild] = React.useState<BuildingKey | null>(null);
    const [loadingBase, setLoadingBase] = React.useState(true);
    const [loadingHeroes, setLoadingHeroes] = React.useState(true);
    const [baseError, setBaseError] = React.useState("");
    const [heroesError, setHeroesError] = React.useState("");
    const [notice, setNotice] = React.useState("");
    const [busyAction, setBusyAction] = React.useState<string | null>(null);
    const busyRef = React.useRef(false);
    const baseRequestRef = React.useRef(0);
    const heroesRequestRef = React.useRef(0);

    const loadBase = React.useCallback(async (): Promise<BaseState | null> => {
        if (!user?.token) { setLoadingBase(false); return null; }
        const requestId = ++baseRequestRef.current;
        setLoadingBase(true);
        try {
            const response = await fetch(`${API_BASE}/game/base/state`, { headers: { Authorization: `Bearer ${user.token}` }, cache: "no-store" });
            if (!response.ok) throw new Error("Failed to load Base state");
            const state = readBaseState(await response.json());
            if (requestId !== baseRequestRef.current) return null;
            setBaseState(state); setBaseError("");
            return state;
        } catch (error) { if (requestId === baseRequestRef.current) setBaseError(error instanceof Error ? error.message : "Failed to load Base state"); return null; }
        finally { if (requestId === baseRequestRef.current) setLoadingBase(false); }
    }, [user?.token]);

    const fetchHeroes = React.useCallback(async (): Promise<HeroesState> => {
        if (!user?.token) throw new Error("Session unavailable");
        const response = await fetch(`${API_BASE}/game/heroes`, { headers: { Authorization: `Bearer ${user.token}` }, cache: "no-store" });
        if (!response.ok) throw new Error("Failed to load heroes");
        return readHeroesState(await response.json());
    }, [user?.token]);

    const loadHeroes = React.useCallback(async (): Promise<HeroesState | null> => {
        if (!user?.token) { setLoadingHeroes(false); return null; }
        const requestId = ++heroesRequestRef.current;
        setLoadingHeroes(true);
        try {
            const state = await fetchHeroes();
            if (requestId !== heroesRequestRef.current) return null;
            setHeroesState(state); setHeroesError("");
            return state;
        } catch (error) { if (requestId === heroesRequestRef.current) setHeroesError(error instanceof Error ? error.message : "Failed to load heroes"); return null; }
        finally { if (requestId === heroesRequestRef.current) setLoadingHeroes(false); }
    }, [fetchHeroes, user?.token]);

    React.useEffect(() => {
        setBaseState(null);
        setHeroesState(null);
        setBaseError("");
        setHeroesError("");
        void loadBase();
        void loadHeroes();
    }, [loadBase, loadHeroes]);

    const build = async () => {
        const key = confirmBuild;
        if (!key || !baseState || !user?.token || busyRef.current || baseError || !buildActionAllowed(baseState[key], false)) return;
        busyRef.current = true; setBusyAction(`build:${key}`); setBaseError(""); setNotice("");
        try {
            const response = await fetch(`${API_BASE}${BUILDINGS[key].path}`, { method: "POST", headers: { Authorization: `Bearer ${user.token}` } });
            if (!response.ok) throw new Error(requestError(await response.text(), `Failed to build ${BUILDINGS[key].name}`));
            let confirmed: BaseState | null = null;
            try { confirmed = readBaseState(await response.json()); } catch { /* A fresh GET below verifies the action. */ }
            const refreshed = await loadBase();
            const latest = refreshed || confirmed;
            if (!latest?.[key].built) throw new Error(`Server did not confirm ${BUILDINGS[key].name} construction`);
            if (!refreshed) setBaseState(latest);
            await loadHeroes();
            setConfirmBuild(null);
            setNotice(refreshed ? `${BUILDINGS[key].name} built.` : `${BUILDINGS[key].name} built. Retry to refresh the latest Base state.`);
        } catch (error) { setBaseError(error instanceof Error ? error.message : "Building failed"); }
        finally { busyRef.current = false; setBusyAction(null); }
    };

    const changeHero = async (hero: HeroState, action: "hire" | "activate") => {
        if (!user?.token || !heroesState || busyRef.current || (action === "hire" ? heroAction(hero) !== "hire" : heroAction(hero) !== "activate")) return;
        busyRef.current = true; setBusyAction(`${action}:${hero.id}`); setHeroesError(""); setNotice("");
        ++heroesRequestRef.current;
        try {
            const path = action === "hire" ? `/game/heroes/${encodeURIComponent(hero.id)}/hire` : "/game/heroes/active";
            const response = await fetch(`${API_BASE}${path}`, { method: "POST", headers: { Authorization: `Bearer ${user.token}`, ...(action === "activate" ? { "Content-Type": "application/json" } : {}) }, ...(action === "activate" ? { body: JSON.stringify({ heroClassId: hero.id }) } : {}) });
            if (!response.ok) throw new Error(requestError(await response.text(), `Failed to ${action} ${hero.displayName}`));
            let confirmed: HeroesState | null = null;
            try { confirmed = readHeroesState(await response.json()); } catch { /* A fresh GET below verifies the action. */ }
            if (!confirmed || !confirmedHeroAction(heroesState, confirmed, hero.id, action)) confirmed = await fetchHeroes();
            if (!confirmedHeroAction(heroesState, confirmed, hero.id, action)) throw new Error("Server did not confirm the hero change");
            setHeroesState(confirmed);
            if (action === "hire") await loadBase();
            setNotice(action === "hire" ? `${hero.displayName} hired.` : `${hero.displayName} is now active.`);
            void loadHeroes();
        } catch (error) { setHeroesError(error instanceof Error ? error.message : "Hero action failed"); }
        finally { busyRef.current = false; setBusyAction(null); }
    };

    const building = baseState ? selectedBuilding(baseState, selected) : null;
    const buildStatus = building ? buildingStatus(building) : null;
    const pending = confirmBuild && baseState ? baseState[confirmBuild] : null;
    const buildDescription = pending && confirmBuild ? `Current level ${pending.level}. Build ${BUILDINGS[confirmBuild].name} for ${RESOURCE_KEYS.map(key => `${pending.costs[key]} ${key}`).join(", ")}?` : "";

    return <main className={styles.pageRoot}><div className={styles.container}>
        <header className={styles.pageHeader}><span className={styles.eyebrow}><UiText text="YOUR STRONGHOLD" /></span><h1><UiText text="Base" /></h1><p><UiText text="Build your halls, gather knowledge, and prepare your heroes." /></p></header>
        <ResourceSummary state={baseState} gold={heroesState?.gold ?? null} />
        {baseError && <StatusMessage tone="error">{baseError} <button type="button" onClick={() => void loadBase()}><UiText text="Retry" /></button></StatusMessage>}
        {heroesError && <StatusMessage tone="error">{heroesError} <button type="button" onClick={() => void loadHeroes()}><UiText text="Retry heroes" /></button></StatusMessage>}
        {notice && <StatusMessage tone="success">{notice}</StatusMessage>}
        {loadingBase && !baseState ? <LoadingState title="Opening the stronghold…" /> : !baseState ? <ErrorState title="Base unavailable" description="Retry loading the current server state." action={<button type="button" className={styles.secondaryButton} onClick={() => void loadBase()}><UiText text="Retry" /></button>} /> : <>
            <BuildingSelector state={baseState} selected={selected} onSelect={setSelected} />
            {building && <section className={styles.workPanel} aria-label={`${BUILDINGS[selected].name} details`}>
                <div className={`${styles.workArt} ${styles[selected]}`}><span className={styles.eyebrow}><UiText text="SELECTED BUILDING" /></span><h2><UiText text={BUILDINGS[selected].name} /></h2><span><UiText text="Level" /> {building.level}</span></div>
                <div className={styles.workContent}>
                    <div className={styles.workHeading}><div><span className={styles.eyebrow}><UiText text="STRONGHOLD DEVELOPMENT" /></span><p><UiText text={BUILDINGS[selected].description} /></p></div><span className={styles.statusBadge}>{buildStatus ? <UiText text={STATUS_LABELS[buildStatus]} /> : "—"}</span></div>
                    {!building.built ? <><h3><UiText text="Construction cost" /></h3><BuildingCost building={building} />{buildStatus === "unavailable" && <p className={styles.blockReason}><UiText text="Construction is unavailable under the current server rules." /></p>}<button type="button" className={styles.primaryButton} disabled={!buildActionAllowed(building, busyAction !== null || Boolean(baseError))} onClick={() => setConfirmBuild(selected)}><UiText text={busyAction === `build:${selected}` ? "Building…" : "Build"} /> <UiText text={BUILDINGS[selected].name} /></button></> : selected !== "tavern" && <p className={styles.builtNote}><UiText text="Built and ready. This building has no available upgrade action." /></p>}
                    {selected === "forge" && building.built && <div className={styles.featureSection}><div className={styles.featureHeading}><h3><UiText text="Blueprint references" /></h3><span><UiText text="Information only" /></span></div>{building.unlockables.length ? <div className={styles.referenceGrid}>{building.unlockables.map(recipe => <article key={recipe.id}><strong><UiText text={recipe.name} /></strong><p><UiText text={recipe.description} /></p></article>)}</div> : <p><UiText text="No blueprint references available." /></p>}<p className={styles.featureNote}><UiText text="Crafting is not available in Base." /></p></div>}
                    {selected === "library" && <div className={styles.featureSection}><div className={styles.featureHeading}><h3><UiText text="Library notes" /></h3><span><UiText text="Coming Soon" /></span></div>{building.unlockables.length > 0 && <div className={styles.referenceGrid}>{building.unlockables.map(entry => <article key={entry.id}><strong><UiText text={entry.name} /></strong><p><UiText text={entry.description} /></p></article>)}</div>}<p className={styles.featureNote}><UiText text="Research actions are not available yet." /></p></div>}
                    {selected === "tavern" && <div className={styles.featureSection}><div className={styles.featureHeading}><h3><UiText text="Hero roster" /></h3><span>{heroesState?.heroes.filter(hero => hero.owned).length ?? "—"} <UiText text="owned" /></span></div>{!building.built && <p className={styles.featureNote}><UiText text="Build the Tavern to hire new heroes." /></p>}{loadingHeroes && !heroesState ? <LoadingState title="Loading heroes…" /> : heroesState?.heroes.length ? <div className={styles.heroGrid}>{heroesState.heroes.map(hero => <TavernHeroCard key={hero.id} hero={hero} busy={busyAction === `hire:${hero.id}` || busyAction === `activate:${hero.id}`} onHire={candidate => void changeHero(candidate, "hire")} onActivate={candidate => void changeHero(candidate, "activate")} />)}</div> : <EmptyState title="No heroes available" description="The server has not returned a hero roster." />}</div>}
                </div>
            </section>}
        </>}
        <Link href="/market" className={styles.marketBuilding} aria-label="Открыть Аукционный дом"><img src="/ui-icons/market.svg" alt="" /><span><strong>Аукционный дом</strong><small>Аукцион и биржа игроков</small></span><span aria-hidden="true">→</span></Link>
        <aside className={styles.loadoutRow}><div className={styles.loadoutIntro}><span className={styles.eyebrow}><UiText text="MATCH PREPARATION" /></span><Link href="/mode"><UiText text="Edit in Modes ↗" /></Link></div><StorageLoadout compact collapsible /></aside>
    </div><ConfirmDialog open={Boolean(confirmBuild && pending)} title={confirmBuild ? `Build ${BUILDINGS[confirmBuild].name}?` : "Build?"} description={buildDescription} confirmLabel="Confirm build" busy={busyAction !== null} onConfirm={() => void build()} onCancel={() => { if (!busyRef.current) setConfirmBuild(null); }} /></main>;
}
