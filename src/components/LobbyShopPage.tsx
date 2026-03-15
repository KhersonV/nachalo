"use client";

import React from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "../contexts/AuthContext";
import LobbyHeader from "./LobbyHeader";
import type { PlayerShopState, ShopItem } from "../types";
import styles from "../styles/ModeSelectionPage.module.css";

const API_GAME = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8001";

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

    const loadShopData = React.useCallback(async () => {
        if (!user) return;
        try {
            const [itemsRes, playerRes] = await Promise.all([
                fetch(`${API_GAME}/game/shop/items`),
                fetch(`${API_GAME}/game/player/${user.id}`),
            ]);

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
        } catch (e) {
            console.error("failed to load shop data", e);
        }
    }, [user]);

    const getItemCountFromInventory = React.useCallback(
        (itemId: number) => {
            if (!shopPlayer?.inventory) return 0;
            try {
                const parsed =
                    typeof shopPlayer.inventory === "string"
                        ? JSON.parse(shopPlayer.inventory)
                        : shopPlayer.inventory;
                const key = `resource_${itemId}`;
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
                setShopError("Недостаточно монет для покупки");
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
                    throw new Error(text || "Ошибка покупки");
                }

                const data = await res.json();
                setShopPlayer({
                    user_id: data.player.user_id,
                    balance: Number(data.player.balance ?? 0),
                    inventory: data.player.inventory ?? "{}",
                });
                setShopInfo(`Куплено: ${item.name} x${safeCount}`);
            } catch (e: unknown) {
                const message =
                    e instanceof Error
                        ? e.message
                        : "Не удалось купить предмет";
                setShopError(message);
            } finally {
                setShopBusyType(null);
            }
        },
        [user, shopPlayer],
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
            <h2 className={styles.pageTitle}>Магазин</h2>

            <section className={styles.shopPanel}>
                <div className={styles.shopHeader}>
                    <h3 className={styles.shopTitle}>Магазин подготовки</h3>
                    <p className={styles.shopSubtitle}>
                        Пока стоишь в очереди, можно закупиться едой и водой
                        перед стартом.
                    </p>
                    <div className={styles.shopBalance}>
                        Баланс: {shopPlayer?.balance ?? 0}
                    </div>
                </div>

                {shopError && <p className={styles.shopError}>{shopError}</p>}
                {shopInfo && <p className={styles.shopInfo}>{shopInfo}</p>}

                <div className={styles.shopGrid}>
                    {shopItems.map((item) => {
                        const owned = getItemCountFromInventory(item.id);
                        const currentCount = shopCounts[item.type] ?? 1;
                        const totalCost = item.price * currentCount;
                        const canBuy = (shopPlayer?.balance ?? 0) >= totalCost;

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
                                        Эффект:{" "}
                                        {item.effect.health
                                            ? `+${item.effect.health} HP`
                                            : ""}
                                        {item.effect.energy
                                            ? `+${item.effect.energy} Energy`
                                            : ""}
                                    </span>
                                    <span>Цена за 1: {item.price}</span>
                                    <span
                                        className={
                                            canBuy
                                                ? styles.shopCostOk
                                                : styles.shopCostWarn
                                        }
                                    >
                                        Сумма: {totalCost}
                                    </span>
                                    <span>В инвентаре: {owned}</span>
                                </div>

                                <div className={styles.shopQtyRow}>
                                    <span className={styles.shopQtyLabel}>
                                        Кол-во:
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
                                        ? "Покупка..."
                                        : "Купить"}
                                </button>
                            </article>
                        );
                    })}
                </div>
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
