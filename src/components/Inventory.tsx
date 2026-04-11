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

interface InventoryProps {
    onBlueprintPlacementStart?: (inventoryKey: string) => void;
    onRequestClose?: () => void;
}

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
    return normalizedName === "food" || normalizedName === "water";
};

const Inventory: React.FC<InventoryProps> = ({
    onBlueprintPlacementStart,
    onRequestClose,
}) => {
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
    if (!player) return <div>Inventory unavailable</div>;

    // Images are provided by the server in inventory entries; use them directly.

    const normalize = (it: any, keyHint?: string): RawInventoryItem => {
        let hintType: "resource" | "artifact" | undefined;
        let hintId: number | undefined;
        if (keyHint) {
            const [t, id] = keyHint.split("_");
            if ((t === "resource" || t === "artifact" || t === "scroll") && !isNaN(+id)) {
                hintType = t as any;
                hintId = +id;
            }
        }

        const explicitItemType =
            it.item_type === "resource" || it.item_type === "artifact" || it.item_type === "scroll"
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
            inventory_key: keyHint,
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
                const key = it.inventory_key || `${it.item_type}_${it.item_id}`;
                if (it.item_type === "resource" || it.item_type === "scroll") acc.resources[key] = it;
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

        if (key.startsWith("blueprint_")) {
            onBlueprintPlacementStart?.(key);
            onRequestClose?.();
            setFeedback("Build mode: select an adjacent free cell.");
            return;
        }

        // Scrolls: call inventory use endpoint with item_type = "scroll" and show scroll result (if any)
        if (key.startsWith("scroll_")) {
            // continue to use the generic inventory use endpoint; backend will return scroll_result
        }

        setFeedback("");

        const [item_type, idStr] = key.split("_");
        const item_id = parseInt(idStr, 10);

        try {
            const stored = localStorage.getItem("user");
            const token = stored ? JSON.parse(stored).token : "";
            if (!token) {
                setFeedback("Session not found. Please log in again.");
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
                        count: 1,
                    }),
                },
            );
            if (!res.ok) {
                const errorText = await res.text();
                setFeedback(errorText || "Failed to use item");
                return;
            }
            const data = await res.json();
            // backend may return either PlayerState or { player: PlayerState, scroll_result: { axis, value } }
            let updatedPlayer: PlayerState | null = null;
            if (data && data.player) {
                updatedPlayer = data.player as PlayerState;
                if (data.scroll_result) {
                    setFeedback(
                        `Revealed ${String(data.scroll_result.axis).toUpperCase()}: ${data.scroll_result.value}`,
                    );
                }
            } else {
                updatedPlayer = data as PlayerState;
            }
            if (updatedPlayer) {
                dispatch(updatePlayer({ instanceId, player: updatedPlayer }));
            }
        } catch (err) {
            setFeedback("Network error while using item");
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
                    {(() => {
                        // For blueprints, prefer showing the built-structure image
                        const isBlueprint = key.startsWith("blueprint_");
                        const blueprintName = isBlueprint
                            ? key.replace("blueprint_", "")
                            : null;
                        const displayImage = isBlueprint
                            ? `/Forge-items/${blueprintName}.png`
                            : item.image || "";

                        return (
                            <img
                                src={displayImage}
                                alt={item.name}
                                className={styles.itemImage}
                                loading="lazy"
                            />
                        );
                    })()}
                    <div className={styles.itemName}>{item.name}</div>
                    {section === "resources" && (
                        <>
                            <div className={styles.itemCount}>
                                Qty: {item.item_count}
                            </div>
                            {(isConsumableResource(item) ||
                                key.startsWith("blueprint_") ||
                                key.startsWith("scroll_") ||
                                item.item_type === "scroll") && (
                                <button
                                    className={styles.useButton}
                                    onClick={() =>
                                        handleUseItem(section, key, item)
                                    }
                                >
                                    {key.startsWith("blueprint_")
                                        ? "Place"
                                        : "Use"}
                                </button>
                            )}
                        </>
                    )}
                </div>
            ));

    return (
        <div className={styles.inventoryContainer}>
            <h2 className={styles.title}>Inventory</h2>
            {feedback && (
                <div className={styles.feedbackRow}>
                    <p className={styles.feedbackError}>{feedback}</p>
                    <button
                        type="button"
                        className={styles.feedbackCloseButton}
                        onClick={() => setFeedback("")}
                        aria-label="Close message"
                        title="Close"
                    >
                        x
                    </button>
                </div>
            )}
            <div className={styles.sections}>
                <div className={styles.section}>
                    <h3 className={styles.sectionTitle}>Resources</h3>
                    <p className={styles.sectionHint}>
                        Limits: food - 1 time per 2 turns, water - up to 5 per 2
                        turns.
                    </p>
                    <div className={styles.itemsContainer}>
                        {renderItems("resources", resources)}
                    </div>
                </div>
                <div className={styles.section}>
                    <h3 className={styles.sectionTitle}>Artifacts</h3>
                    <div className={styles.itemsContainer}>
                        {renderItems("artifacts", artifacts)}
                    </div>
                </div>
            </div>
        </div>
    );
};

export default Inventory;
