"use client";

import React from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "../contexts/AuthContext";
import LobbyHeader from "./LobbyHeader";
import type { BaseState, BuildingState } from "../types";
import styles from "../styles/ModeSelectionPage.module.css";

const API_GAME = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8001";

export default function LobbyBasePage() {
    const router = useRouter();
    const { user } = useAuth();

    const [baseState, setBaseState] = React.useState<BaseState | null>(null);
    const [baseBusy, setBaseBusy] = React.useState(false);
    const [baseError, setBaseError] = React.useState("");
    const [baseInfo, setBaseInfo] = React.useState("");

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

    const renderBuildingSection = React.useCallback(
        (
            title: string,
            subtitle: string,
            building: BuildingState,
            onBuild: () => void,
            buildLabel: string,
            unlockablesTitle: string,
        ) => (
            <section className={styles.basePanel}>
                <div className={styles.baseHeader}>
                    <h3 className={styles.baseTitle}>{title}</h3>
                    <p className={styles.baseSubtitle}>{subtitle}</p>
                </div>

                <div className={styles.baseStatusRow}>
                    <span>
                        Status: {building.built ? "Built" : "Not built"}
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
            </section>
        ),
        [baseBusy],
    );

    React.useEffect(() => {
        loadBaseState();
    }, [loadBaseState]);

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
