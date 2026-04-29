"use client";

import React from "react";
import { useRouter } from "next/navigation";
import { API_BASE as API_GAME } from "@/utils/serviceUrls";
import { useAuth } from "../contexts/AuthContext";
import LobbyHeader from "./LobbyHeader";
import type { BaseState, BuildingState, HeroesState, HeroState } from "../types";
import styles from "../styles/ModeSelectionPage.module.css";

const isRecord = (value: unknown): value is Record<string, unknown> =>
    typeof value === "object" && value !== null;

const readString = (
    source: Record<string, unknown>,
    key: string,
    fallback = "",
) => {
    const value = source[key];
    return typeof value === "string" ? value : fallback;
};

const readNumber = (
    source: Record<string, unknown>,
    key: string,
    fallback = 0,
) => {
    const value = source[key];
    return typeof value === "number" && Number.isFinite(value)
        ? value
        : fallback;
};

const readBoolean = (
    source: Record<string, unknown>,
    key: string,
    fallback = false,
) => {
    const value = source[key];
    return typeof value === "boolean" ? value : fallback;
};

const normalizeHero = (value: unknown, index: number): HeroState | null => {
    if (!isRecord(value)) return null;
    const id = readString(value, "id");
    if (!id) return null;

    const displayName = readString(value, "displayName", id);
    const lockReason = value.lockReason;

    return {
        id,
        displayName,
        description: readString(value, "description"),
        owned: readBoolean(value, "owned"),
        active: readBoolean(value, "active"),
        locked: readBoolean(value, "locked", !readBoolean(value, "owned")),
        unlockPrice: readNumber(value, "unlockPrice"),
        requiresTavern: readBoolean(value, "requiresTavern"),
        canHire: readBoolean(value, "canHire"),
        lockReason: typeof lockReason === "string" ? lockReason : null,
        enabled: readBoolean(value, "enabled", true),
        sortOrder: readNumber(value, "sortOrder", index),
    };
};

const normalizeHeroesState = (value: unknown): HeroesState => {
    const source = isRecord(value) ? value : {};
    const heroesRaw = Array.isArray(source.heroes) ? source.heroes : [];
    const heroes = heroesRaw
        .map((entry, index) => normalizeHero(entry, index))
        .filter((entry): entry is HeroState => Boolean(entry))
        .sort((a, b) => a.sortOrder - b.sortOrder);

    return {
        activeHeroClassId: readString(source, "activeHeroClassId"),
        tavernBuilt: readBoolean(source, "tavernBuilt"),
        gold: readNumber(source, "gold"),
        heroes,
    };
};

const heroErrorMessage = (responseText: string, fallback: string) => {
    if (responseText.includes("hero_already_owned")) {
        return "Hero is already owned";
    }
    if (responseText.includes("not_enough_gold")) {
        return "Not enough gold to hire this hero";
    }
    if (responseText.includes("tavern_required")) {
        return "Build the Tavern before hiring heroes";
    }
    if (responseText.includes("hero_not_owned")) {
        return "You do not own this hero yet";
    }
    if (responseText.includes("hero_not_found")) {
        return "Hero not found";
    }
    return fallback;
};

const lockReasonLabel = (reason: string | null) => {
    if (reason === "tavern_required") return "Tavern required";
    if (reason === "not_enough_gold") return "Not enough gold";
    return reason ? "Locked" : "";
};

export default function LobbyBasePage() {
    const router = useRouter();
    const { user } = useAuth();

    const [baseState, setBaseState] = React.useState<BaseState | null>(null);
    const [baseBusy, setBaseBusy] = React.useState(false);
    const [baseError, setBaseError] = React.useState("");
    const [baseInfo, setBaseInfo] = React.useState("");
    const [heroesState, setHeroesState] = React.useState<HeroesState | null>(
        null,
    );
    const [heroesLoading, setHeroesLoading] = React.useState(false);
    const [heroesError, setHeroesError] = React.useState("");
    const [heroBusyId, setHeroBusyId] = React.useState<string | null>(null);

    const loadBaseState = React.useCallback(async () => {
        if (!user?.token) return;
        setBaseError("");
        try {
            const res = await fetch(`${API_GAME}/game/base/state`, {
                headers: {
                    Authorization: `Bearer ${user.token}`,
                },
            });
            if (!res.ok) {
                throw new Error("Failed to load base state");
            }
            const data: BaseState = await res.json();
            setBaseState(data);
        } catch (e: unknown) {
            const message =
                e instanceof Error ? e.message : "Failed to load base";
            setBaseError(message);
        }
    }, [user]);

    const loadHeroes = React.useCallback(async () => {
        if (!user?.token) return;
        setHeroesLoading(true);
        setHeroesError("");
        try {
            const res = await fetch(`${API_GAME}/game/heroes`, {
                headers: {
                    Authorization: `Bearer ${user.token}`,
                },
            });

            if (!res.ok) {
                throw new Error("Failed to load Tavern heroes");
            }

            const data = normalizeHeroesState(await res.json());
            setHeroesState(data);
        } catch (e: unknown) {
            const message =
                e instanceof Error
                    ? e.message
                    : "Failed to load Tavern heroes";
            setHeroesError(message);
            setHeroesState(null);
        } finally {
            setHeroesLoading(false);
        }
    }, [user]);

    const handleBuildForge = React.useCallback(async () => {
        if (!user?.token) return;
        setBaseBusy(true);
        setBaseError("");
        setBaseInfo("");
        try {
            const res = await fetch(`${API_GAME}/game/base/forge/build`, {
                method: "POST",
                headers: {
                    Authorization: `Bearer ${user.token}`,
                },
            });

            if (!res.ok) {
                const text = await res.text();
                if (text.includes("not_enough_resources")) {
                    throw new Error(
                        "Not enough resources to build the forge",
                    );
                }
                if (text.includes("forge_already_built")) {
                    throw new Error("Forge is already built");
                }
                throw new Error("Failed to build the forge");
            }

            const data: BaseState = await res.json();
            setBaseState(data);
            setBaseInfo("Forge built. New recipes unlocked.");
        } catch (e: unknown) {
            const message =
                e instanceof Error ? e.message : "Forge build error";
            setBaseError(message);
        } finally {
            setBaseBusy(false);
        }
    }, [user]);

    const handleBuildLibrary = React.useCallback(async () => {
        if (!user?.token) return;
        setBaseBusy(true);
        setBaseError("");
        setBaseInfo("");
        try {
            const res = await fetch(`${API_GAME}/game/base/library/build`, {
                method: "POST",
                headers: {
                    Authorization: `Bearer ${user.token}`,
                },
            });

            if (!res.ok) {
                const text = await res.text();
                if (text.includes("not_enough_resources")) {
                    throw new Error(
                        "Not enough resources to build the library",
                    );
                }
                if (text.includes("library_already_built")) {
                    throw new Error("Library is already built");
                }
                throw new Error("Failed to build the library");
            }

            const data: BaseState = await res.json();
            setBaseState(data);
            setBaseInfo("Library built. Research unlocked.");
        } catch (e: unknown) {
            const message =
                e instanceof Error ? e.message : "Library build error";
            setBaseError(message);
        } finally {
            setBaseBusy(false);
        }
    }, [user]);

    const handleBuildTavern = React.useCallback(async () => {
        if (!user?.token) return;
        setBaseBusy(true);
        setBaseError("");
        setBaseInfo("");
        try {
            const res = await fetch(`${API_GAME}/game/base/tavern/build`, {
                method: "POST",
                headers: {
                    Authorization: `Bearer ${user.token}`,
                },
            });

            if (!res.ok) {
                const text = await res.text();
                if (text.includes("not_enough_resources")) {
                    throw new Error(
                        "Not enough resources to build the tavern",
                    );
                }
                if (text.includes("tavern_already_built")) {
                    throw new Error("Tavern is already built");
                }
                throw new Error("Failed to build the tavern");
            }

            const data: BaseState = await res.json();
            setBaseState(data);
            setBaseInfo("Tavern built. Hero hiring unlocked.");
        } catch (e: unknown) {
            const message =
                e instanceof Error ? e.message : "Tavern build error";
            setBaseError(message);
        } finally {
            setBaseBusy(false);
        }
    }, [user]);

    const handleHireHero = React.useCallback(
        async (hero: HeroState) => {
            if (!user?.token || heroBusyId) return;
            setHeroBusyId(`hire:${hero.id}`);
            setHeroesError("");
            setBaseInfo("");
            try {
                const res = await fetch(
                    `${API_GAME}/game/heroes/${encodeURIComponent(hero.id)}/hire`,
                    {
                        method: "POST",
                        headers: {
                            Authorization: `Bearer ${user.token}`,
                        },
                    },
                );

                if (!res.ok) {
                    const text = await res.text();
                    throw new Error(
                        heroErrorMessage(text, "Failed to hire hero"),
                    );
                }

                const data = normalizeHeroesState(await res.json());
                setHeroesState(data);
                setBaseInfo(`${hero.displayName} hired.`);
            } catch (e: unknown) {
                const message =
                    e instanceof Error ? e.message : "Failed to hire hero";
                setHeroesError(message);
            } finally {
                setHeroBusyId(null);
            }
        },
        [heroBusyId, user],
    );

    const handleSetActiveHero = React.useCallback(
        async (hero: HeroState) => {
            if (!user?.token || heroBusyId) return;
            setHeroBusyId(`active:${hero.id}`);
            setHeroesError("");
            setBaseInfo("");
            try {
                const res = await fetch(`${API_GAME}/game/heroes/active`, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${user.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify({ heroClassId: hero.id }),
                });

                if (!res.ok) {
                    const text = await res.text();
                    throw new Error(
                        heroErrorMessage(text, "Failed to set active hero"),
                    );
                }

                const data = normalizeHeroesState(await res.json());
                setHeroesState(data);
                setBaseInfo(`${hero.displayName} set as active hero.`);
            } catch (e: unknown) {
                const message =
                    e instanceof Error
                        ? e.message
                        : "Failed to set active hero";
                setHeroesError(message);
            } finally {
                setHeroBusyId(null);
            }
        },
        [heroBusyId, user],
    );

    const renderBuildingSection = React.useCallback(
        (
            title: string,
            subtitle: string,
            building: BuildingState,
            onBuild: () => void,
            buildLabel: string,
            unlockablesTitle: string,
            statusLabels?: { built: string; notBuilt: string },
            extraContent?: React.ReactNode,
        ) => (
            <section className={styles.basePanel}>
                <div className={styles.baseHeader}>
                    <h3 className={styles.baseTitle}>{title}</h3>
                    <p className={styles.baseSubtitle}>{subtitle}</p>
                </div>

                <div className={styles.baseStatusRow}>
                    <span>
                        Status:{" "}
                        {building.built
                            ? (statusLabels?.built ?? "Built")
                            : (statusLabels?.notBuilt ?? "Not built")}
                    </span>
                    <span>Level: {building.level}</span>
                </div>

                <div className={styles.baseResourcesGrid}>
                    <div className={styles.baseResourceCard}>
                        <strong>Wood</strong>
                        <span>
                            {building.resources.wood} / {building.costs.wood}
                        </span>
                    </div>
                    <div className={styles.baseResourceCard}>
                        <strong>Stone</strong>
                        <span>
                            {building.resources.stone} / {building.costs.stone}
                        </span>
                    </div>
                    <div className={styles.baseResourceCard}>
                        <strong>Iron</strong>
                        <span>
                            {building.resources.iron} / {building.costs.iron}
                        </span>
                    </div>
                </div>

                {!building.built && (
                    <button
                        className={styles.baseBuildButton}
                        onClick={onBuild}
                        disabled={!building.canBuild || baseBusy}
                    >
                        {baseBusy ? "Building..." : buildLabel}
                    </button>
                )}

                {building.built && building.unlockables.length > 0 && (
                    <div className={styles.baseRecipes}>
                        <h4>{unlockablesTitle}</h4>
                        <ul>
                            {building.unlockables.map((entry) => {
                                // Known building blueprints — translate known names/descriptions
                                let desc = entry.description;
                                let displayName = entry.name;
                                if (entry.name === "Башня разведки") {
                                    displayName = "Watchtower";
                                    desc =
                                        "Increases player's sight by 1 cell (max 5). Only one watchtower can be built.";
                                } else if (entry.name === "Турель") {
                                    displayName = "Turret";
                                    desc =
                                        "Automatically attacks monsters near the base, consumes energy. Priority: players, then monsters.";
                                } else if (entry.name === "Библиотека") {
                                    displayName = "Library";
                                    desc = "Opens research for character development.";
                                } else if (entry.name === "Кузница") {
                                    displayName = "Forge";
                                    desc =
                                        "Allows crafting new blueprints and items to prepare for matches.";
                                }
                                return (
                                    <li key={entry.id}>
                                        <strong>{displayName}</strong>: {desc}
                                    </li>
                                );
                            })}
                        </ul>
                    </div>
                )}

                {extraContent}
            </section>
        ),
        [baseBusy],
    );

    const renderTavernHeroManagement = React.useCallback(() => {
        if (!baseState?.tavern.built) return null;

        return (
            <div className={styles.tavernHeroSection}>
                <div className={styles.tavernHeroHeader}>
                    <div>
                        <h4>Tavern</h4>
                        <p>
                            Set your active hero before starting a match. Earn
                            gold from matches to hire new heroes.
                        </p>
                    </div>
                    <strong className={styles.tavernGold}>
                        Gold: {heroesState?.gold ?? "—"}
                    </strong>
                </div>

                {heroesLoading && (
                    <p className={styles.baseSubtitle}>Loading heroes...</p>
                )}
                {heroesError && (
                    <p className={styles.baseError}>{heroesError}</p>
                )}

                {!heroesLoading &&
                    !heroesError &&
                    heroesState &&
                    heroesState.heroes.length === 0 && (
                        <p className={styles.baseSubtitle}>
                            No heroes available yet.
                        </p>
                    )}

                {heroesState && heroesState.heroes.length > 0 && (
                    <div className={styles.tavernHeroGrid}>
                        {heroesState.heroes.map((hero) => {
                            const lockLabel = lockReasonLabel(
                                hero.lockReason,
                            );
                            const busyHire = heroBusyId === `hire:${hero.id}`;
                            const busyActive =
                                heroBusyId === `active:${hero.id}`;
                            const actionDisabled = heroBusyId !== null;

                            return (
                                <article
                                    key={hero.id}
                                    className={styles.tavernHeroCard}
                                >
                                    <div className={styles.tavernHeroCardTop}>
                                        <h5>{hero.displayName}</h5>
                                        <div className={styles.tavernBadges}>
                                            {hero.active ? (
                                                <span
                                                    className={`${styles.tavernBadge} ${styles.tavernBadgeActive}`}
                                                >
                                                    Active
                                                </span>
                                            ) : hero.owned ? (
                                                <span
                                                    className={`${styles.tavernBadge} ${styles.tavernBadgeOwned}`}
                                                >
                                                    Owned
                                                </span>
                                            ) : (
                                                <span
                                                    className={`${styles.tavernBadge} ${styles.tavernBadgeLocked}`}
                                                >
                                                    Locked
                                                </span>
                                            )}
                                        </div>
                                    </div>

                                    <p className={styles.tavernHeroText}>
                                        {hero.description ||
                                            "A hero ready for future runs."}
                                    </p>

                                    {!hero.owned && (
                                        <div className={styles.tavernHeroMeta}>
                                            {hero.unlockPrice > 0 && (
                                                <span>
                                                    Cost: {hero.unlockPrice}{" "}
                                                    gold
                                                </span>
                                            )}
                                            {lockLabel && (
                                                <span>{lockLabel}</span>
                                            )}
                                        </div>
                                    )}

                                    <div className={styles.tavernHeroActions}>
                                        {hero.active ? (
                                            <button
                                                className={
                                                    styles.tavernHeroButton
                                                }
                                                disabled
                                            >
                                                Active
                                            </button>
                                        ) : hero.owned ? (
                                            <button
                                                className={
                                                    styles.tavernHeroButton
                                                }
                                                onClick={() =>
                                                    handleSetActiveHero(hero)
                                                }
                                                disabled={actionDisabled}
                                            >
                                                {busyActive
                                                    ? "Setting..."
                                                    : "Set Active"}
                                            </button>
                                        ) : hero.canHire ? (
                                            <button
                                                className={
                                                    styles.tavernHeroButton
                                                }
                                                onClick={() =>
                                                    handleHireHero(hero)
                                                }
                                                disabled={actionDisabled}
                                            >
                                                {busyHire ? "Hiring..." : "Hire"}
                                            </button>
                                        ) : (
                                            <button
                                                className={
                                                    styles.tavernHeroButton
                                                }
                                                disabled
                                            >
                                                Locked
                                            </button>
                                        )}
                                    </div>
                                </article>
                            );
                        })}
                    </div>
                )}
            </div>
        );
    }, [
        baseState?.tavern.built,
        handleHireHero,
        handleSetActiveHero,
        heroBusyId,
        heroesError,
        heroesLoading,
        heroesState,
    ]);

    React.useEffect(() => {
        loadBaseState();
    }, [loadBaseState]);

    React.useEffect(() => {
        if (baseState?.tavern.built) {
            loadHeroes();
            return;
        }

        setHeroesState(null);
        setHeroesError("");
        setHeroesLoading(false);
    }, [baseState?.tavern.built, loadHeroes]);

    return (
        <div className={styles.pageRoot}>
            <LobbyHeader />
            <h2 className={styles.pageTitle}>Base</h2>

            {baseError && <p className={styles.baseError}>{baseError}</p>}
            {baseInfo && <p className={styles.baseInfo}>{baseInfo}</p>}

            {baseState ? (
                <>
                    {renderBuildingSection(
                        "Base: Forge",
                        "The forge is built in the lobby and unlocks new recipes for match preparation.",
                        baseState.forge,
                        handleBuildForge,
                        "Build Forge",
                        "Unlocked Recipes",
                    )}

                    {renderBuildingSection(
                        "Base: Library",
                        "The library is built in the lobby and unlocks research for character development.",
                        baseState.library,
                        handleBuildLibrary,
                        "Build Library",
                        "Unlocked Research",
                    )}

                    {renderBuildingSection(
                        "Tavern",
                        "Hire heroes and try new playstyles.",
                        baseState.tavern,
                        handleBuildTavern,
                        "Build Tavern",
                        "Tavern Ready",
                        {
                            built: "Tavern built",
                            notBuilt: "Build Tavern",
                        },
                        renderTavernHeroManagement(),
                    )}
                </>
            ) : (
                <section className={styles.basePanel}>
                    <p className={styles.baseSubtitle}>Loading base...</p>
                </section>
            )}

            <div className={styles.buttonGroup}>
                <button
                    className={styles.queueButton}
                    onClick={() => router.push("/mode")}
                >
                    Back to Modes
                </button>
            </div>
        </div>
    );
}
