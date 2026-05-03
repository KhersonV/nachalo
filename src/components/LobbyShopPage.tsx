"use client";

import React from "react";
import { useRouter } from "next/navigation";
import { API_BASE as API_GAME } from "@/utils/serviceUrls";
import { useAuth } from "../contexts/AuthContext";
import LobbyHeader from "./LobbyHeader";
import type { PlayerShopState, ShopItem } from "../types";
import type { EquipmentItem } from "../types/equipment";
import styles from "../styles/ModeSelectionPage.module.css";

const EFFECT_LABELS: Record<string, string> = {
    health: "HP",
    energy: "Energy",
    sight_bonus: "Sight",
    structure_health: "Structure HP",
    structure_defense: "Structure Defense",
    turret_damage: "Turret Damage",
    structure_blocking: "Blocks Passage",
};
function formatBonuses(bonuses?: Record<string, number>) {
    if (!bonuses) return "";
    return Object.entries(bonuses)
        .filter(([, v]) => typeof v === "number" && v !== 0)
        .map(([k, v]) => `${k}: ${v > 0 ? "+" + v : v}`)
        .join("; ");
}

function formatEnum(value: string | undefined) {
    if (!value) return "";
    return value
        .split("_")
        .filter(Boolean)
        .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
        .join(" ");
}

// images are provided by server; no client-side blueprint suffixing required

function formatEffect(effect: Record<string, number>): string {
    const entries = Object.entries(effect || {});
    if (!entries.length) return "None";
    return entries
        .map(([key, value]) => {
            const label = EFFECT_LABELS[key] ?? key;
            if (key === "structure_blocking") {
                return `${label}: ${value > 0 ? "Yes" : "No"}`;
            }
            const sign = value > 0 ? "+" : "";
            return `${label}: ${sign}${value}`;
        })
        .join("; ");
}

export default function LobbyShopPage() {
    const router = useRouter();
    const { user } = useAuth();

    const [shopItems, setShopItems] = React.useState<ShopItem[]>([]);
    const [shopPlayer, setShopPlayer] = React.useState<PlayerShopState | null>(
        null,
    );
    const [shopCounts, setShopCounts] = React.useState<Record<string, number>>(
        {},
    );
    const [shopBusyType, setShopBusyType] = React.useState<string | null>(null);
    const [shopError, setShopError] = React.useState("");
    const [shopInfo, setShopInfo] = React.useState("");
    const [sellItems, setSellItems] = React.useState<EquipmentItem[]>([]);
    const [sellBusyId, setSellBusyId] = React.useState<string | null>(null);
    const [sellInfo, setSellInfo] = React.useState("");
    const [forgeBuilt, setForgeBuilt] = React.useState(false);
    const [libraryBuilt, setLibraryBuilt] = React.useState(false);
    const [activeTab, setActiveTab] = React.useState<"buy" | "sell">("buy");

    const loadShopData = React.useCallback(async () => {
        if (!user) return;
        try {
            const [itemsRes, playerRes] = await Promise.all([
                fetch(`${API_GAME}/game/shop/items`),
                fetch(`${API_GAME}/game/player/${user.id}`),
            ]);

            if (user?.token) {
                const baseRes = await fetch(`${API_GAME}/game/base/state`, {
                    headers: {
                        Authorization: `Bearer ${user.token}`,
                    },
                });
                if (baseRes.ok) {
                    const baseData = await baseRes.json();
                    setForgeBuilt(
                        Boolean(baseData?.forge?.built ?? baseData?.built),
                    );
                    setLibraryBuilt(
                        Boolean(
                            baseData?.library?.built ??
                            baseData?.Library?.built,
                        ),
                    );
                }
            }

            if (itemsRes.ok) {
                const itemsData = await itemsRes.json();
                const items: ShopItem[] = itemsData.items || [];
                setShopItems(items);
                setShopCounts((prev) => {
                    const next = { ...prev };
                    for (const item of items) {
                        if (!next[item.type] || next[item.type] < 1) {
                            next[item.type] = 1;
                        }
                    }
                    return next;
                });
            }

            if (playerRes.ok) {
                const playerData = await playerRes.json();
                setShopPlayer({
                    user_id: playerData.user_id,
                    balance: Number(playerData.balance ?? 0),
                    inventory: playerData.inventory ?? "{}",
                });
            }

            // load equipment sellable items (unequipped owned items)
            try {
                if (user?.token) {
                    const eqRes = await fetch(`${API_GAME}/game/equipment`, {
                        headers: { Authorization: `Bearer ${user.token}` },
                    });
                    if (eqRes.ok) {
                        const eqData = await eqRes.json();
                        const raw = eqData?.data || {};
                        const source = Array.isArray(raw.inventoryItems)
                            ? raw.inventoryItems
                            : Array.isArray(raw.ownedItems)
                            ? raw.ownedItems
                            : raw.inventory || [];
                        const items: EquipmentItem[] = (source as any[])
                            .filter((it) => it && it.status !== "equipped" && !it.equippedCharacterId)
                            .map((it) => it as EquipmentItem);
                        setSellItems(items);
                    }
                }
            } catch (e) {
                console.error("failed to load equipment for sell list", e);
            }
        } catch (e) {
            console.error("failed to load shop data", e);
        }
    }, [user]);

    const getItemCountFromInventory = React.useCallback(
        (item: ShopItem) => {
            if (!shopPlayer?.inventory) return 0;
            try {
                const parsed =
                    typeof shopPlayer.inventory === "string"
                        ? JSON.parse(shopPlayer.inventory)
                        : shopPlayer.inventory;
                const key = item.inventoryKey || `resource_${item.id}`;
                const entry = parsed?.[key];
                if (!entry) return 0;
                const val = entry.item_count;
                if (typeof val === "number") return val;
                if (typeof val === "string") return Number(val) || 0;
                return 0;
            } catch {
                return 0;
            }
        },
        [shopPlayer],
    );

    const handleBuyShopItem = React.useCallback(
        async (item: ShopItem, count: number) => {
            if (!user?.token || !shopPlayer) return;
            const safeCount = Math.max(1, Math.min(99, count));
            const totalCost = item.price * safeCount;
            if (shopPlayer.balance < totalCost) {
                setShopError("Not enough coins to buy");
                setShopInfo("");
                return;
            }

            setShopBusyType(item.type);
            setShopError("");
            setShopInfo("");

            try {
                const res = await fetch(`${API_GAME}/game/shop/buy`, {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        Authorization: `Bearer ${user.token}`,
                    },
                    body: JSON.stringify({
                        player_id: user.id,
                        item_type: item.type,
                        count: safeCount,
                    }),
                });

                if (!res.ok) {
                    const text = await res.text();
                    if (text.includes("forge_required")) {
                        throw new Error(
                            "Forge required to purchase blueprints",
                        );
                    }
                    throw new Error(text || "Purchase error");
                }

                const data = await res.json();
                setShopPlayer({
                    user_id: data.player.user_id,
                    balance: Number(data.player.balance ?? 0),
                    inventory: data.player.inventory ?? "{}",
                });
                setShopInfo(`Purchased: ${item.name} x${safeCount}`);
            } catch (e: unknown) {
                const message =
                    e instanceof Error ? e.message : "Failed to buy item";
                setShopError(message);
            } finally {
                setShopBusyType(null);
            }
        },
        [user, shopPlayer],
    );

    const handleSellEquipment = React.useCallback(
        async (item: EquipmentItem) => {
            if (!user?.token) return;
            if (!confirm(`Sell ${item.name} for gold?`)) return;

            setSellBusyId(item.instanceId);
            setShopError("");
            setSellInfo("");

            try {
                const res = await fetch(`${API_GAME}/game/equipment/sell`, {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        Authorization: `Bearer ${user.token}`,
                    },
                    body: JSON.stringify({ itemInstanceId: item.instanceId }),
                });

                if (!res.ok) {
                    const text = await res.text();
                    if (text.includes("item_is_equipped")) {
                        throw new Error("Unequip this item before selling it.");
                    }
                    throw new Error(text || "Failed to sell item");
                }

                const data = await res.json();

                if (data?.balance !== undefined) {
                    setShopPlayer((prev) => (prev ? { ...prev, balance: Number(data.balance) } : prev));
                } else if (data?.player && data.player.balance !== undefined) {
                    setShopPlayer({
                        user_id: data.player.user_id,
                        balance: Number(data.player.balance ?? 0),
                        inventory: data.player.inventory ?? "{}",
                    });
                } else {
                    // fallback: refresh player
                    const pr = await fetch(`${API_GAME}/game/player/${user.id}`);
                    if (pr.ok) {
                        const pd = await pr.json();
                        setShopPlayer({ user_id: pd.user_id, balance: Number(pd.balance ?? 0), inventory: pd.inventory ?? "{}" });
                    }
                }

                setSellItems((prev) => prev.filter((i) => i.instanceId !== item.instanceId));
                setSellInfo(`Sold: ${item.name} for ${data?.goldDelta ?? item.sellPrice ?? 0} gold`);
            } catch (e: unknown) {
                const message = e instanceof Error ? e.message : String(e);
                setShopError(message);
            } finally {
                setSellBusyId(null);
            }
        },
        [user],
    );

    const setShopCount = React.useCallback(
        (itemType: string, count: number) => {
            const safe = Number.isFinite(count)
                ? Math.max(1, Math.min(99, Math.floor(count)))
                : 1;
            setShopCounts((prev) => ({ ...prev, [itemType]: safe }));
        },
        [],
    );

    const increaseShopCount = React.useCallback(
        (itemType: string, delta: number) => {
            setShopCounts((prev) => {
                const current = prev[itemType] ?? 1;
                const safe = Math.max(1, Math.min(99, current + delta));
                return { ...prev, [itemType]: safe };
            });
        },
        [],
    );

    const setMaxAffordableCount = React.useCallback(
        (itemType: string, unitPrice: number) => {
            const balance = shopPlayer?.balance ?? 0;
            if (unitPrice <= 0) return;
            const maxByBalance = Math.floor(balance / unitPrice);
            const next = Math.max(1, Math.min(99, maxByBalance));
            setShopCounts((prev) => ({ ...prev, [itemType]: next }));
        },
        [shopPlayer],
    );

    React.useEffect(() => {
        loadShopData();
    }, [loadShopData]);

    return (
        <div className={styles.pageRoot}>
            <LobbyHeader />
            <h2 className={styles.pageTitle}>Shop</h2>

            <div style={{ display: "flex", gap: "0.5rem", marginTop: "0.75rem" }}>
                <button
                    className={styles.partyActionButton}
                    onClick={() => setActiveTab("buy")}
                    disabled={activeTab === "buy"}
                >
                    Buy
                </button>
                <button
                    className={styles.partyActionButton}
                    onClick={() => setActiveTab("sell")}
                    disabled={activeTab === "sell"}
                >
                    Sell
                </button>
            </div>

            {activeTab === "buy" && (
                <section className={styles.shopPanel}>
                <div className={styles.shopHeader}>
                    <h3 className={styles.shopTitle}>Preparation Shop</h3>
                    <p className={styles.shopSubtitle}>
                        Waiting for the match to start.
                    </p>
                    <div className={styles.shopBalance}>
                        Balance: {shopPlayer?.balance ?? 0}
                    </div>
                </div>

                {shopError && <p className={styles.shopError}>{shopError}</p>}
                {shopInfo && <p className={styles.shopInfo}>{shopInfo}</p>}

                <div className={styles.shopGrid}>
                    {shopItems.map((item) => {
                        const owned = getItemCountFromInventory(item);
                        const currentCount = shopCounts[item.type] ?? 1;
                        const totalCost = item.price * currentCount;
                        const canAfford =
                            (shopPlayer?.balance ?? 0) >= totalCost;
                        const blockedByForge =
                            (item.requiresForge && !forgeBuilt) ||
                            (item.category === "scroll" && !libraryBuilt);
                        const canBuy = canAfford && !blockedByForge;

                        return (
                            <article
                                key={item.type}
                                className={styles.shopCard}
                            >
                                <img
                                    src={item.image}
                                    alt={item.name}
                                    className={styles.shopImage}
                                />
                                <div className={styles.shopMeta}>
                                    <strong>{item.name}</strong>
                                    <span>{item.description}</span>
                                    <span>
                                        Effect: {formatEffect(item.effect)}
                                    </span>
                                    <span>Price per 1: {item.price}</span>
                                    {item.requiresForge && !forgeBuilt && (
                                        <span className={styles.shopCostWarn}>
                                            Forge required
                                        </span>
                                    )}
                                    {item.category === "scroll" &&
                                        !libraryBuilt && (
                                            <span
                                                className={styles.shopCostWarn}
                                            >
                                                Library required
                                            </span>
                                        )}
                                    <span
                                        className={
                                            canAfford
                                                ? styles.shopCostOk
                                                : styles.shopCostWarn
                                        }
                                    >
                                        Total: {totalCost}
                                    </span>
                                    <span>In inventory: {owned}</span>
                                </div>

                                <div className={styles.shopQtyRow}>
                                    <span className={styles.shopQtyLabel}>
                                        Qty:
                                    </span>
                                    <div className={styles.shopQtyActions}>
                                        <button
                                            type="button"
                                            className={styles.shopQtyButton}
                                            onClick={() =>
                                                increaseShopCount(item.type, -1)
                                            }
                                            disabled={
                                                !!shopBusyType ||
                                                currentCount <= 1
                                            }
                                        >
                                            -
                                        </button>
                                        <input
                                            className={styles.shopQtyInput}
                                            type="number"
                                            min={1}
                                            max={99}
                                            value={currentCount}
                                            onChange={(e) =>
                                                setShopCount(
                                                    item.type,
                                                    Number(e.target.value),
                                                )
                                            }
                                            disabled={!!shopBusyType}
                                        />
                                        <button
                                            type="button"
                                            className={styles.shopQtyButton}
                                            onClick={() =>
                                                increaseShopCount(item.type, 1)
                                            }
                                            disabled={
                                                !!shopBusyType ||
                                                currentCount >= 99
                                            }
                                        >
                                            +
                                        </button>
                                    </div>
                                    <div className={styles.shopQtyQuick}>
                                        <button
                                            type="button"
                                            className={styles.shopQuickButton}
                                            onClick={() =>
                                                increaseShopCount(item.type, 1)
                                            }
                                            disabled={
                                                !!shopBusyType ||
                                                currentCount >= 99
                                            }
                                        >
                                            +1
                                        </button>
                                        <button
                                            type="button"
                                            className={styles.shopQuickButton}
                                            onClick={() =>
                                                increaseShopCount(item.type, 5)
                                            }
                                            disabled={
                                                !!shopBusyType ||
                                                currentCount >= 99
                                            }
                                        >
                                            +5
                                        </button>
                                        <button
                                            type="button"
                                            className={styles.shopQuickButton}
                                            onClick={() =>
                                                setMaxAffordableCount(
                                                    item.type,
                                                    item.price,
                                                )
                                            }
                                            disabled={
                                                !!shopBusyType ||
                                                (shopPlayer?.balance ?? 0) <
                                                    item.price
                                            }
                                        >
                                            Max
                                        </button>
                                    </div>
                                </div>

                                <button
                                    className={styles.shopBuyButton}
                                    disabled={!canBuy || !!shopBusyType}
                                    onClick={() =>
                                        handleBuyShopItem(item, currentCount)
                                    }
                                >
                                    {shopBusyType === item.type
                                        ? "Buying..."
                                        : blockedByForge
                                          ? "Forge required"
                                          : "Buy"}
                                </button>
                            </article>
                        );
                    })}
                </div>
                </section>
            )}

            {activeTab === "sell" && (
                <section className={styles.panel}>
                    <div className={styles.panelHeader}>
                    <h2>Sell Equipment</h2>
                    <span>{sellItems.length} sellable</span>
                </div>

                {shopError ? (
                    <div className={styles.shopError}>{shopError}</div>
                ) : null}
                {sellInfo ? <div className={styles.shopInfo}>{sellInfo}</div> : null}

                {sellItems.length === 0 ? (
                    <div className={styles.emptyState}>No items to sell. Unequip equipment first or find items from monsters.</div>
                ) : (
                    <div className={styles.shopGrid}>
                        {sellItems.map((item) => {
                            const busy = sellBusyId === item.instanceId;
                            return (
                                <article
                                    className={styles.shopCard}
                                    key={item.instanceId}
                                >
                                    <img
                                        src={item.imageUrl || (item as any).image || ""}
                                        alt={item.name}
                                        className={styles.shopImage}
                                    />
                                    <div className={styles.shopMeta}>
                                        <strong>{item.name}</strong>
                                        <span>
                                            {item.rarity} · {item.classRestriction || "Any Class"} · {item.setName || item.setCode || ""}
                                        </span>
                                        <span>Slot: {formatEnum(item.slot)}</span>
                                        <span>{formatBonuses(item.bonuses as any)}</span>
                                        <span>Sell for {item.sellPrice ?? 0} gold</span>
                                        <button
                                            type="button"
                                            className={styles.shopBuyButton}
                                            onClick={() => handleSellEquipment(item)}
                                            disabled={busy}
                                        >
                                            {busy ? "Selling..." : `Sell for ${item.sellPrice ?? 0} gold`}
                                        </button>
                                    </div>
                                </article>
                            );
                        })}
                    </div>
                )}
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
