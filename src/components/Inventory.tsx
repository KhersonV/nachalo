//==================================
// src/components/Inventory.tsx
//==================================

"use client";

import React, { useEffect, useMemo, useState } from "react";
import { useDispatch, useSelector } from "react-redux";
import { useAuth } from "../contexts/AuthContext";
import type { RootState } from "../store";
import type { RawInventoryItem, PlayerState } from "../types";
import { updatePlayer } from "../store/slices/gameSlice";
import styles from "../styles/Inventory.module.css";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8001";

const isConsumableResource = (item: RawInventoryItem): boolean => {
    if (item.item_type !== "resource") return false;
    const effect = item.effect || {};
    if (
        typeof effect.health === "number" ||
        typeof effect.energy === "number"
    ) {
        return true;
    }

    const normalizedName = String(item.name || "")
        .trim()
        .toLowerCase();
    return (
        normalizedName === "food" ||
        normalizedName === "water" ||
        normalizedName === "еда" ||
        normalizedName === "вода"
    );
};

const Inventory: React.FC = () => {
    const dispatch = useDispatch();
    const state = useSelector((state: RootState) => state.game);
    const { user } = useAuth();
    const instanceId = state.instanceId;
    const [feedback, setFeedback] = useState("");

    useEffect(() => {
        if (!feedback) return;
        const timer = window.setTimeout(() => {
            setFeedback("");
        }, 4000);
        return () => window.clearTimeout(timer);
    }, [feedback]);

    const player = state.players.find((p) => p.user_id === user?.id);
    if (!player) return <div>Инвентарь недоступен</div>;

    const normalize = (it: any, keyHint?: string): RawInventoryItem => {
        let hintType: "resource" | "artifact" | undefined;
        let hintId: number | undefined;
        if (keyHint) {
            const [t, id] = keyHint.split("_");
            if ((t === "resource" || t === "artifact") && !isNaN(+id)) {
                hintType = t;
                hintId = +id;
            }
        }

        const explicitItemType =
            it.item_type === "resource" || it.item_type === "artifact"
                ? it.item_type
                : undefined;
        const item_type = (explicitItemType || hintType || "resource") as
            | "resource"
            | "artifact";
        const item_id = it.item_id || it.id || hintId || 0;
        const name = it.name || it.item_name || item_type + "_" + item_id;
        const item_count = it.item_count || it.count || 1;
        const image = it.image || it.image_url || "";
        const description = it.description || it.item_description || "";

        return {
            item_type,
            item_id,
            name,
            item_count,
            image,
            description,
            bonus: it.bonus,
            effect: it.effect,
        };
    };

    const rawItems: RawInventoryItem[] = useMemo(() => {
        let parsed: any;
        try {
            parsed =
                typeof player.inventory === "string"
                    ? JSON.parse(player.inventory)
                    : player.inventory;
        } catch {
            return [];
        }

        let flat: any[] = [];
        if (Array.isArray(parsed)) {
            flat = parsed;
        } else if (parsed.resources || parsed.artifacts) {
            flat = [
                ...(parsed.resources ? Object.entries(parsed.resources) : []),
                ...(parsed.artifacts ? Object.entries(parsed.artifacts) : []),
            ].map(([key, val]) => ({ val, key }));
        } else {
            flat = Object.entries(parsed).map(([key, val]) => ({ val, key }));
        }

        return flat.map((entry) => {
            if ((entry as any).val !== undefined) {
                return normalize((entry as any).val, (entry as any).key);
            } else {
                return normalize(entry);
            }
        });
    }, [player.inventory]);

    const { resources, artifacts } = useMemo(() => {
        return rawItems.reduce<{
            resources: Record<string, RawInventoryItem>;
            artifacts: Record<string, RawInventoryItem>;
        }>(
            (acc, it) => {
                const key = `${it.item_type}_${it.item_id}`;
                if (it.item_type === "resource") acc.resources[key] = it;
                else acc.artifacts[key] = it;
                return acc;
            },
            { resources: {}, artifacts: {} },
        );
    }, [rawItems]);

    const handleUseItem = async (
        section: "resources" | "artifacts",
        key: string,
        item: RawInventoryItem,
    ) => {
        if (item.item_count <= 0 || !user) return;

        setFeedback("");

        const [item_type, idStr] = key.split("_");
        const item_id = parseInt(idStr, 10);

        try {
            const stored = localStorage.getItem("user");
            const token = stored ? JSON.parse(stored).token : "";
            if (!token) {
                setFeedback("Сессия не найдена. Войдите снова.");
                return;
            }

            const res = await fetch(
                `${API_BASE}/game/player/${user.id}/inventory/use`,
                {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        Authorization: `Bearer ${token}`,
                    },
                    body: JSON.stringify({
                        instance_id: instanceId,
                        item_type,
                        item_id,
                        item_count: 1,
                    }),
                },
            );
            if (!res.ok) {
                const errorText = await res.text();
                setFeedback(errorText || "Не удалось использовать предмет");
                return;
            }
            const updatedPlayer = (await res.json()) as PlayerState;
            dispatch(updatePlayer({ instanceId, player: updatedPlayer }));
        } catch (err) {
            setFeedback("Ошибка сети при использовании предмета");
        }
    };

    const renderItems = (
        section: "resources" | "artifacts",
        items: Record<string, RawInventoryItem>,
    ) =>
        Object.entries(items)
            .filter(([, item]) => item.item_count > 0)
            .map(([key, item]) => (
                <div key={key} className={styles.item}>
                    <img
                        src={item.image}
                        alt={item.name}
                        className={styles.itemImage}
                        loading="lazy"
                    />
                    <div className={styles.itemName}>{item.name}</div>
                    {section === "resources" && (
                        <>
                            <div className={styles.itemCount}>
                                Кол-во: {item.item_count}
                            </div>
                            {isConsumableResource(item) && (
                                <button
                                    className={styles.useButton}
                                    onClick={() =>
                                        handleUseItem(section, key, item)
                                    }
                                >
                                    Использовать
                                </button>
                            )}
                        </>
                    )}
                </div>
            ));

    return (
        <div className={styles.inventoryContainer}>
            <h2 className={styles.title}>Инвентарь</h2>
            {feedback && (
                <div className={styles.feedbackRow}>
                    <p className={styles.feedbackError}>{feedback}</p>
                    <button
                        type="button"
                        className={styles.feedbackCloseButton}
                        onClick={() => setFeedback("")}
                        aria-label="Закрыть сообщение"
                        title="Закрыть"
                    >
                        x
                    </button>
                </div>
            )}
            <div className={styles.sections}>
                <div className={styles.section}>
                    <h3 className={styles.sectionTitle}>Ресурсы</h3>
                    <p className={styles.sectionHint}>
                        Лимиты: еда - 1 раз в 2 хода, вода - до 5 за 2 хода.
                    </p>
                    <div className={styles.itemsContainer}>
                        {renderItems("resources", resources)}
                    </div>
                </div>
                <div className={styles.section}>
                    <h3 className={styles.sectionTitle}>Артефакты</h3>
                    <div className={styles.itemsContainer}>
                        {renderItems("artifacts", artifacts)}
                    </div>
                </div>
            </div>
        </div>
    );
};

export default Inventory;
