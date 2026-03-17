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
                throw new Error("Не удалось загрузить состояние базы");
            }
            const data: BaseState = await res.json();
            setBaseState(data);
        } catch (e: unknown) {
            const message =
                e instanceof Error ? e.message : "Не удалось загрузить базу";
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
                        "Недостаточно ресурсов для постройки кузницы",
                    );
                }
                if (text.includes("forge_already_built")) {
                    throw new Error("Кузница уже построена");
                }
                throw new Error("Не удалось построить кузницу");
            }

            const data: BaseState = await res.json();
            setBaseState(data);
            setBaseInfo("Кузница построена. Новые рецепты открыты.");
        } catch (e: unknown) {
            const message =
                e instanceof Error ? e.message : "Ошибка постройки кузницы";
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
                        "Недостаточно ресурсов для постройки библиотеки",
                    );
                }
                if (text.includes("library_already_built")) {
                    throw new Error("Библиотека уже построена");
                }
                throw new Error("Не удалось построить библиотеку");
            }

            const data: BaseState = await res.json();
            setBaseState(data);
            setBaseInfo("Библиотека построена. Открыты исследования.");
        } catch (e: unknown) {
            const message =
                e instanceof Error ? e.message : "Ошибка постройки библиотеки";
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
                        Статус: {building.built ? "Построена" : "Не построена"}
                    </span>
                    <span>Уровень: {building.level}</span>
                </div>

                <div className={styles.baseResourcesGrid}>
                    <div className={styles.baseResourceCard}>
                        <strong>Дерево</strong>
                        <span>
                            {building.resources.wood} / {building.costs.wood}
                        </span>
                    </div>
                    <div className={styles.baseResourceCard}>
                        <strong>Камень</strong>
                        <span>
                            {building.resources.stone} / {building.costs.stone}
                        </span>
                    </div>
                    <div className={styles.baseResourceCard}>
                        <strong>Железо</strong>
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
                        {baseBusy ? "Строим..." : buildLabel}
                    </button>
                )}

                {building.built && building.unlockables.length > 0 && (
                    <div className={styles.baseRecipes}>
                        <h4>{unlockablesTitle}</h4>
                        <ul>
                            {building.unlockables.map((entry) => {
                                // Известные чертежи строений
                                let desc = entry.description;
                                if (entry.name === "Башня разведки") {
                                    desc =
                                        "Увеличивает обзор игрока на 1 клетку (максимум 5). Можно построить только одну башню.";
                                } else if (entry.name === "Турель") {
                                    desc =
                                        "Автоматически атакует монстров рядом с базой, расходует энергию. Приоритет: игроки, затем монстры.";
                                } else if (entry.name === "Библиотека") {
                                    desc =
                                        "Открывает исследования для развития персонажа.";
                                } else if (entry.name === "Кузница") {
                                    desc =
                                        "Позволяет создавать новые чертежи и предметы для подготовки к матчам.";
                                }
                                return (
                                    <li key={entry.id}>
                                        <strong>{entry.name}</strong>: {desc}
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
            <h2 className={styles.pageTitle}>База</h2>

            {baseError && <p className={styles.baseError}>{baseError}</p>}
            {baseInfo && <p className={styles.baseInfo}>{baseInfo}</p>}

            {baseState ? (
                <>
                    {renderBuildingSection(
                        "База: Кузница",
                        "Кузница строится в лобби и открывает новые рецепты для подготовки к матчам.",
                        baseState.forge,
                        handleBuildForge,
                        "Построить кузницу",
                        "Открытые рецепты",
                    )}

                    {renderBuildingSection(
                        "База: Библиотека",
                        "Библиотека строится в лобби и открывает исследования для развития персонажа.",
                        baseState.library,
                        handleBuildLibrary,
                        "Построить библиотеку",
                        "Открытые исследования",
                    )}
                </>
            ) : (
                <section className={styles.basePanel}>
                    <p className={styles.baseSubtitle}>Загружаем базу...</p>
                </section>
            )}

            <div className={styles.buttonGroup}>
                <button
                    className={styles.queueButton}
                    onClick={() => router.push("/mode")}
                >
                    К режимам
                </button>
            </div>
        </div>
    );
}
