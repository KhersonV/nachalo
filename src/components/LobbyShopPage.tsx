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

// Canonical shop pricing for green equipment (display-only while backend support is pending)
const GREEN_NORMAL_PRICE = 2000;
const GREEN_TWO_HANDED_PRICE = 4000;

type GreenItem = {
    type: string;
    name: string;
    description?: string;
    image?: string;
    slot?: string;
    rarity?: string;
    setName: string;
    classRestriction: string;
    isTwoHanded?: boolean;
    price?: number;
};

const GREEN_SETS: { setCode: string; setName: string; className: string; items: GreenItem[] }[] = [
    {
        setCode: "sagecloth",
        setName: "Sagecloth Set",
        className: "Mystic",
        items: [
            { type: "sagecloth_staff", name: "Sagecloth Staff", image: "/equipment/mystic/sagecloth/staff.png", slot: "main_hand", rarity: "green", setName: "Sagecloth", classRestriction: "Mystic", isTwoHanded: true },
            { type: "sagecloth_hood", name: "Sagecloth Hood", image: "/equipment/mystic/sagecloth/hood.png", slot: "helmet", rarity: "green", setName: "Sagecloth", classRestriction: "Mystic" },
            { type: "sagecloth_jacket", name: "Sagecloth Jacket", image: "/equipment/mystic/sagecloth/jacket.png", slot: "chest", rarity: "green", setName: "Sagecloth", classRestriction: "Mystic" },
            { type: "sagecloth_pants", name: "Sagecloth Pants", image: "/equipment/mystic/sagecloth/pants.png", slot: "pants", rarity: "green", setName: "Sagecloth", classRestriction: "Mystic" },
            { type: "sagecloth_gloves", name: "Sagecloth Gloves", image: "/equipment/mystic/sagecloth/gloves.png", slot: "gloves", rarity: "green", setName: "Sagecloth", classRestriction: "Mystic" },
            { type: "sagecloth_boots", name: "Sagecloth Boots", image: "/equipment/mystic/sagecloth/boots.png", slot: "boots", rarity: "green", setName: "Sagecloth", classRestriction: "Mystic" },
        ],
    },
    {
        setCode: "bloodroot",
        setName: "Bloodroot Set",
        className: "Berserker",
        items: [
            { type: "bloodroot_axe", name: "Bloodroot Axe", image: "/equipment/berserker/bloodroot/Bloodroot Axe.png", slot: "main_hand", rarity: "green", setName: "Bloodroot", classRestriction: "Berserker", isTwoHanded: true },
            { type: "bloodroot_helm", name: "Bloodroot Horned Helm", image: "/equipment/berserker/bloodroot/Bloodroot Horned Helm.png", slot: "helmet", rarity: "green", setName: "Bloodroot", classRestriction: "Berserker" },
            { type: "bloodroot_vest", name: "Bloodroot War Vest", image: "/equipment/berserker/bloodroot/Bloodroot War Vest.png", slot: "chest", rarity: "green", setName: "Bloodroot", classRestriction: "Berserker" },
            { type: "bloodroot_pants", name: "Bloodroot Raider Pants", image: "/equipment/berserker/bloodroot/Bloodroot Raider Pants.png", slot: "pants", rarity: "green", setName: "Bloodroot", classRestriction: "Berserker" },
            { type: "bloodroot_gloves", name: "Bloodroot Grips", image: "/equipment/berserker/bloodroot/Bloodroot Grips.png", slot: "gloves", rarity: "green", setName: "Bloodroot", classRestriction: "Berserker" },
            { type: "bloodroot_boots", name: "Bloodroot Stompers", image: "/equipment/berserker/bloodroot/Bloodroot Stompers.png", slot: "boots", rarity: "green", setName: "Bloodroot", classRestriction: "Berserker" },
        ],
    },
    {
        setCode: "greenwisp",
        setName: "Greenwisp Set",
        className: "Ranger",
        items: [
            { type: "greenwisp_bow", name: "Greenwisp Bow", image: "/equipment/ranger/greenwisp/Greenwisp Bow.png", slot: "main_hand", rarity: "green", setName: "Greenwisp", classRestriction: "Ranger", isTwoHanded: true },
            { type: "greenwisp_hood", name: "Greenwisp Hood", image: "/equipment/ranger/greenwisp/Greenwisp Hood.png", slot: "helmet", rarity: "green", setName: "Greenwisp", classRestriction: "Ranger" },
            { type: "greenwisp_vest", name: "Greenwisp Vest", image: "/equipment/ranger/greenwisp/Greenwisp Vest.png", slot: "chest", rarity: "green", setName: "Greenwisp", classRestriction: "Ranger" },
            { type: "greenwisp_pants", name: "Greenwisp Ranger Pants", image: "/equipment/ranger/greenwisp/Greenwisp Ranger Pants.png", slot: "pants", rarity: "green", setName: "Greenwisp", classRestriction: "Ranger" },
            { type: "greenwisp_gloves", name: "Greenwisp Gloves", image: "/equipment/ranger/greenwisp/Greenwisp Gloves.png", slot: "gloves", rarity: "green", setName: "Greenwisp", classRestriction: "Ranger" },
            { type: "greenwisp_boots", name: "Greenwisp Boots", image: "/equipment/ranger/greenwisp/Greenwisp Boots.png", slot: "boots", rarity: "green", setName: "Greenwisp", classRestriction: "Ranger" },
        ],
    },
    {
        setCode: "aegiswarden",
        setName: "Aegiswarden Set",
        className: "Guardian",
        items: [
            { type: "aegis_sword", name: "Aegiswarden Sword", image: "/equipment/guardian/aegiswarden/Aegiswarden Sword.png", slot: "main_hand", rarity: "green", setName: "Aegiswarden", classRestriction: "Guardian" },
            { type: "aegis_helm", name: "Aegiswarden Helm", image: "/equipment/guardian/aegiswarden/Aegiswarden Helm.png", slot: "helmet", rarity: "green", setName: "Aegiswarden", classRestriction: "Guardian" },
            { type: "aegis_gloves", name: "Aegiswarden Gloves", image: "/equipment/guardian/aegiswarden/Aegiswarden Gloves.png", slot: "gloves", rarity: "green", setName: "Aegiswarden", classRestriction: "Guardian" },
            { type: "aegis_boots", name: "Aegiswarden Boots", image: "/equipment/guardian/aegiswarden/boots.png", slot: "boots", rarity: "green", setName: "Aegiswarden", classRestriction: "Guardian" },
        ],
    },
];

function getGreenItemPrice(it: GreenItem) {
    if (typeof it.price === "number") return it.price;
    return it.isTwoHanded ? GREEN_TWO_HANDED_PRICE : GREEN_NORMAL_PRICE;
}

function formatPrice(price: number) {
    return `${price.toLocaleString()} gold`;
}

function groupConsumables(items: ShopItem[]) {
    const survival: ShopItem[] = [];
    const scrolls: ShopItem[] = [];
    const blueprints: ShopItem[] = [];
    for (const it of items) {
        if (it.category === "resource") {
            if (it.type === "food" || it.type === "water") survival.push(it);
            else survival.push(it);
        } else if (it.category === "scroll") scrolls.push(it);
        else if (it.category === "blueprint") blueprints.push(it);
    }
    return { survival, scrolls, blueprints };
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
    const [forgeBuilt, setForgeBuilt] = React.useState(false);
    const [libraryBuilt, setLibraryBuilt] = React.useState(false);
    const [activeCategory, setActiveCategory] = React.useState<"consumables" | "green_sets">("consumables");
    const [activeTab, setActiveTab] = React.useState<"buy" | "sell">("buy");

    // Equipment / Sell tab state (separate from shop catalog)
    const [equipmentItems, setEquipmentItems] = React.useState<EquipmentItem[]>([]);
    const sellableEquipment = React.useMemo(
        () => equipmentItems.filter((it) => it && it.instanceId && it.status !== "equipped" && !it.equippedCharacterId),
        [equipmentItems],
    );
    const [sellBusyId, setSellBusyId] = React.useState<string | null>(null);
    const [sellInfo, setSellInfo] = React.useState("");

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

            // Note: shop currently provides consumables/blueprints/scrolls.
            // Green equipment sets are shown client-side (see GREEN_SETS below).

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
                        const items: EquipmentItem[] = (source as any[]).map((it) => it as EquipmentItem);
                        setEquipmentItems(items);
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
                // confirm with user
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

                    setEquipmentItems((prev) => prev.filter((i) => i.instanceId !== item.instanceId));
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
                    className={styles.partySecondaryButton}
                    onClick={() => setActiveTab("sell")}
                    disabled={activeTab === "sell"}
                >
                    Sell
                </button>
            </div>

            {activeTab === "buy" && (
                <div style={{ display: "flex", gap: "0.5rem", marginTop: "0.5rem" }}>
                    <button
                        className={styles.partyActionButton}
                        onClick={() => setActiveCategory("consumables")}
                        disabled={activeCategory === "consumables"}
                    >
                        Consumables
                    </button>
                    <button
                        className={styles.partySecondaryButton}
                        onClick={() => setActiveCategory("green_sets")}
                        disabled={activeCategory === "green_sets"}
                    >
                        Green Sets
                    </button>
                </div>
            )}

            {activeTab === "buy" && (
            <section className={styles.shopPanel}>
                <div className={styles.shopHeader}>
                    <h3 className={styles.shopTitle}>Preparation Shop</h3>
                    <p className={styles.shopSubtitle}>Waiting for the match to start.</p>
                    <div className={styles.shopBalance}>Balance: {shopPlayer?.balance ?? 0}</div>
                </div>

                {shopError && <p className={styles.shopError}>{shopError}</p>}
                {shopInfo && <p className={styles.shopInfo}>{shopInfo}</p>}

                {activeCategory === "consumables" && (() => {
                    const { survival, scrolls, blueprints } = groupConsumables(shopItems);
                    return (
                        <>
                            {survival.length > 0 && (
                                <>
                                    <h4 className={styles.baseTitle}>Survival</h4>
                                    <div className={styles.shopGrid}>
                                        {survival.map((item) => {
                                            const owned = getItemCountFromInventory(item);
                                            const currentCount = shopCounts[item.type] ?? 1;
                                            const totalCost = item.price * currentCount;
                                            const canAfford = (shopPlayer?.balance ?? 0) >= totalCost;
                                            const blockedByForge = (item.requiresForge && !forgeBuilt) || (item.category === "scroll" && !libraryBuilt);
                                            const canBuy = canAfford && !blockedByForge;

                                            return (
                                                <article key={item.type} className={styles.shopCard}>
                                                    <img src={item.image} alt={item.name} className={styles.shopImage} />
                                                    <div className={styles.shopMeta}>
                                                        <strong>{item.name}</strong>
                                                        <span className={styles.partyMuted}>{item.description}</span>
                                                        <span>Effect: {formatEffect(item.effect)}</span>
                                                        <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
                                                            <span className={canAfford ? styles.shopCostOk : styles.shopCostWarn}>{formatPrice(item.price)}</span>
                                                            <span>×{currentCount}</span>
                                                        </div>
                                                        <span>In inventory: {owned}</span>
                                                    </div>

                                                    <div className={styles.shopQtyRow}>
                                                        <span className={styles.shopQtyLabel}>Qty:</span>
                                                        <div className={styles.shopQtyActions}>
                                                            <button type="button" className={styles.shopQtyButton} onClick={() => increaseShopCount(item.type, -1)} disabled={!!shopBusyType || currentCount <= 1}>-</button>
                                                            <input className={styles.shopQtyInput} type="number" min={1} max={99} value={currentCount} onChange={(e) => setShopCount(item.type, Number(e.target.value))} disabled={!!shopBusyType} />
                                                            <button type="button" className={styles.shopQtyButton} onClick={() => increaseShopCount(item.type, 1)} disabled={!!shopBusyType || currentCount >= 99}>+</button>
                                                        </div>
                                                        <div className={styles.shopQtyQuick}>
                                                            <button type="button" className={styles.shopQuickButton} onClick={() => increaseShopCount(item.type, 1)} disabled={!!shopBusyType || currentCount >= 99}>+1</button>
                                                            <button type="button" className={styles.shopQuickButton} onClick={() => increaseShopCount(item.type, 5)} disabled={!!shopBusyType || currentCount >= 99}>+5</button>
                                                            <button type="button" className={styles.shopQuickButton} onClick={() => setMaxAffordableCount(item.type, item.price)} disabled={!!shopBusyType || (shopPlayer?.balance ?? 0) < item.price}>Max</button>
                                                        </div>
                                                    </div>

                                                    <button className={styles.shopBuyButton} disabled={!canBuy || !!shopBusyType} onClick={() => handleBuyShopItem(item, currentCount)}>
                                                        {shopBusyType === item.type ? 'Buying...' : blockedByForge ? (item.requiresForge && !forgeBuilt ? 'Forge required' : 'Unavailable') : 'Buy'}
                                                    </button>
                                                </article>
                                            );
                                        })}
                                    </div>
                                </>
                            )}

                            {scrolls.length > 0 && (
                                <>
                                    <h4 className={styles.baseTitle}>Scrolls</h4>
                                    <div className={styles.shopGrid}>
                                        {scrolls.map((item) => {
                                            const currentCount = shopCounts[item.type] ?? 1;
                                            const totalCost = item.price * currentCount;
                                            const canAfford = (shopPlayer?.balance ?? 0) >= totalCost;
                                            const blockedByForge = (item.requiresForge && !forgeBuilt) || (item.category === "scroll" && !libraryBuilt);
                                            return (
                                                <article key={item.type} className={styles.shopCard}>
                                                    <img src={item.image} alt={item.name} className={styles.shopImage} />
                                                    <div className={styles.shopMeta}>
                                                        <strong>{item.name}</strong>
                                                        <span className={styles.partyMuted}>{item.description}</span>
                                                        <span>Effect: {formatEffect(item.effect)}</span>
                                                        <span className={canAfford ? styles.shopCostOk : styles.shopCostWarn}>{formatPrice(item.price)}</span>
                                                    </div>
                                                    <div className={styles.shopQtyRow}>
                                                        <span className={styles.shopQtyLabel}>Qty:</span>
                                                        <div className={styles.shopQtyActions}>
                                                            <button type="button" className={styles.shopQtyButton} onClick={() => increaseShopCount(item.type, -1)} disabled={!!shopBusyType || currentCount <= 1}>-</button>
                                                            <input className={styles.shopQtyInput} type="number" min={1} max={99} value={currentCount} onChange={(e) => setShopCount(item.type, Number(e.target.value))} disabled={!!shopBusyType} />
                                                            <button type="button" className={styles.shopQtyButton} onClick={() => increaseShopCount(item.type, 1)} disabled={!!shopBusyType || currentCount >= 99}>+</button>
                                                        </div>
                                                        <div className={styles.shopQtyQuick}>
                                                            <button type="button" className={styles.shopQuickButton} onClick={() => increaseShopCount(item.type, 1)} disabled={!!shopBusyType || currentCount >= 99}>+1</button>
                                                            <button type="button" className={styles.shopQuickButton} onClick={() => increaseShopCount(item.type, 5)} disabled={!!shopBusyType || currentCount >= 99}>+5</button>
                                                            <button type="button" className={styles.shopQuickButton} onClick={() => setMaxAffordableCount(item.type, item.price)} disabled={!!shopBusyType || (shopPlayer?.balance ?? 0) < item.price}>Max</button>
                                                        </div>
                                                    </div>
                                                    <button className={styles.shopBuyButton} disabled={blockedByForge || !!shopBusyType} onClick={() => handleBuyShopItem(item, currentCount)}>
                                                        {shopBusyType === item.type ? 'Buying...' : blockedByForge ? (item.category === 'scroll' && !libraryBuilt ? 'Library required' : 'Unavailable') : 'Buy'}
                                                    </button>
                                                </article>
                                            );
                                        })}
                                    </div>
                                </>
                            )}

                            {blueprints.length > 0 && (
                                <>
                                    <h4 className={styles.baseTitle}>Towers / Blueprints</h4>
                                    <div className={styles.shopGrid}>
                                        {blueprints.map((item) => {
                                            const currentCount = shopCounts[item.type] ?? 1;
                                            const totalCost = item.price * currentCount;
                                            const canAfford = (shopPlayer?.balance ?? 0) >= totalCost;
                                            const blockedByForge = (item.requiresForge && !forgeBuilt) || (item.category === "scroll" && !libraryBuilt);
                                            return (
                                                <article key={item.type} className={styles.shopCard}>
                                                    <img src={item.image} alt={item.name} className={styles.shopImage} />
                                                    <div className={styles.shopMeta}>
                                                        <strong>{item.name}</strong>
                                                        <span className={styles.partyMuted}>{item.description}</span>
                                                        <span>Effect: {formatEffect(item.effect)}</span>
                                                        <span className={canAfford ? styles.shopCostOk : styles.shopCostWarn}>{formatPrice(item.price)}</span>
                                                    </div>
                                                    <div className={styles.shopQtyRow}>
                                                        <span className={styles.shopQtyLabel}>Qty:</span>
                                                        <div className={styles.shopQtyActions}>
                                                            <button type="button" className={styles.shopQtyButton} onClick={() => increaseShopCount(item.type, -1)} disabled={!!shopBusyType || currentCount <= 1}>-</button>
                                                            <input className={styles.shopQtyInput} type="number" min={1} max={99} value={currentCount} onChange={(e) => setShopCount(item.type, Number(e.target.value))} disabled={!!shopBusyType} />
                                                            <button type="button" className={styles.shopQtyButton} onClick={() => increaseShopCount(item.type, 1)} disabled={!!shopBusyType || currentCount >= 99}>+</button>
                                                        </div>
                                                        <div className={styles.shopQtyQuick}>
                                                            <button type="button" className={styles.shopQuickButton} onClick={() => increaseShopCount(item.type, 1)} disabled={!!shopBusyType || currentCount >= 99}>+1</button>
                                                            <button type="button" className={styles.shopQuickButton} onClick={() => increaseShopCount(item.type, 5)} disabled={!!shopBusyType || currentCount >= 99}>+5</button>
                                                            <button type="button" className={styles.shopQuickButton} onClick={() => setMaxAffordableCount(item.type, item.price)} disabled={!!shopBusyType || (shopPlayer?.balance ?? 0) < item.price}>Max</button>
                                                        </div>
                                                    </div>
                                                    <button className={styles.shopBuyButton} disabled={blockedByForge || !!shopBusyType} onClick={() => handleBuyShopItem(item, currentCount)}>
                                                        {shopBusyType === item.type ? 'Buying...' : blockedByForge ? (item.requiresForge && !forgeBuilt ? 'Forge required' : 'Unavailable') : 'Buy'}
                                                    </button>
                                                </article>
                                            );
                                        })}
                                    </div>
                                </>
                            )}
                        </>
                    );
                })()}

                {activeCategory === "green_sets" && (
                    <>
                        {GREEN_SETS.map((set) => (
                            <div key={set.setCode} style={{ marginTop: '0.8rem' }}>
                                <h4 className={styles.baseTitle}>{set.setName} · {set.className}</h4>
                                <div className={styles.shopGrid}>
                                    {set.items.map((it) => {
                                        const price = getGreenItemPrice(it);
                                        const affordable = (shopPlayer?.balance ?? 0) >= price;
                                        return (
                                            <article key={it.type} className={styles.shopCard}>
                                                {it.image ? <img src={it.image} alt={it.name} className={styles.shopImage} /> : <div style={{ width: 58, height: 58 }} />}
                                                <div className={styles.shopMeta}>
                                                    <strong>{it.name}</strong>
                                                    <span className={styles.partyMuted}>{it.setName} · {it.rarity}</span>
                                                    <span>Slot: {formatEnum(it.slot)}</span>
                                                    <span>Class: {it.classRestriction}</span>
                                                    <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
                                                        <span className={affordable ? styles.shopCostOk : styles.shopCostWarn}>{formatPrice(price)}</span>
                                                        {it.isTwoHanded ? <span className={styles.tavernBadge}>Two-handed</span> : null}
                                                    </div>
                                                </div>
                                                <div style={{ gridColumn: '1 / -1' }}>
                                                    <button className={styles.shopBuyButton} disabled={true} title="Green equipment purchases will be available in a future update">
                                                        Coming soon
                                                    </button>
                                                </div>
                                            </article>
                                        );
                                    })}
                                </div>
                            </div>
                        ))}
                    </>
                )}
            </section>
            )}

            {activeTab === "sell" && (
                <section className={styles.shopPanel}>
                    <div className={styles.shopHeader}>
                        <h3 className={styles.shopTitle}>Sell Equipment</h3>
                        <div className={styles.shopBalance}>{sellableEquipment.length} sellable</div>
                    </div>

                    {shopError ? <p className={styles.shopError}>{shopError}</p> : null}
                    {sellInfo ? <p className={styles.shopInfo}>{sellInfo}</p> : null}

                    {sellableEquipment.length === 0 ? (
                        <div className={styles.emptyState}>No sellable equipment yet. Find or buy equipment first, then return here to sell it.</div>
                    ) : (
                        <div className={styles.shopGrid}>
                            {sellableEquipment.map((item) => {
                                const busy = sellBusyId === item.instanceId;
                                return (
                                    <article className={styles.shopCard} key={item.instanceId}>
                                        <img src={item.imageUrl || (item as any).image || ""} alt={item.name} className={styles.shopImage} />
                                        <div className={styles.shopMeta}>
                                            <strong>{item.name}</strong>
                                            <span className={styles.partyMuted}>{item.rarity} · {item.classRestriction || "Any Class"} · {item.setName || item.setCode || ""}</span>
                                            <span>Slot: {formatEnum(item.slot)}</span>
                                            <span>{formatBonuses(item.bonuses as any)}</span>
                                            <span>Sell for {item.sellPrice ?? 0} gold</span>
                                        </div>
                                        <div style={{ gridColumn: '1 / -1' }}>
                                            <button type="button" className={styles.shopBuyButton} onClick={() => handleSellEquipment(item)} disabled={busy}>
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
