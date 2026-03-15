"use client";

import React from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "../contexts/AuthContext";
import LobbyHeader from "./LobbyHeader";
import type { BaseState } from "../types";
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

    React.useEffect(() => {
        loadBaseState();
    }, [loadBaseState]);

    return (
        <div className={styles.pageRoot}>
            <LobbyHeader />
            <h2 className={styles.pageTitle}>База</h2>

            <section className={styles.basePanel}>
                <div className={styles.baseHeader}>
                    <h3 className={styles.baseTitle}>База: Кузница</h3>
                    <p className={styles.baseSubtitle}>
                        Кузница строится в лобби и открывает новые рецепты для
                        подготовки к матчам.
                    </p>
                </div>

                {baseError && <p className={styles.baseError}>{baseError}</p>}
                {baseInfo && <p className={styles.baseInfo}>{baseInfo}</p>}

                {baseState ? (
                    <>
                        <div className={styles.baseStatusRow}>
                            <span>
                                Статус:{" "}
                                {baseState.built ? "Построена" : "Не построена"}
                            </span>
                            <span>Уровень: {baseState.forgeLevel}</span>
                        </div>

                        <div className={styles.baseResourcesGrid}>
                            <div className={styles.baseResourceCard}>
                                <strong>Дерево</strong>
                                <span>
                                    {baseState.resources.wood} /{" "}
                                    {baseState.costs.wood}
                                </span>
                            </div>
                            <div className={styles.baseResourceCard}>
                                <strong>Камень</strong>
                                <span>
                                    {baseState.resources.stone} /{" "}
                                    {baseState.costs.stone}
                                </span>
                            </div>
                            <div className={styles.baseResourceCard}>
                                <strong>Железо</strong>
                                <span>
                                    {baseState.resources.iron} /{" "}
                                    {baseState.costs.iron}
                                </span>
                            </div>
                        </div>

                        {!baseState.built && (
                            <button
                                className={styles.baseBuildButton}
                                onClick={handleBuildForge}
                                disabled={!baseState.canBuild || baseBusy}
                            >
                                {baseBusy ? "Строим..." : "Построить кузницу"}
                            </button>
                        )}

                        {baseState.built && (
                            <div className={styles.baseRecipes}>
                                <h4>Открытые рецепты</h4>
                                <ul>
                                    {baseState.recipes.map((r) => (
                                        <li key={r.id}>
                                            <strong>{r.name}</strong>:{" "}
                                            {r.description}
                                        </li>
                                    ))}
                                </ul>
                            </div>
                        )}
                    </>
                ) : (
                    <p className={styles.baseSubtitle}>Загружаем базу...</p>
                )}
            </section>

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
