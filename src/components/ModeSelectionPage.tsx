//=====================================
// src/components/ModeSelectionPage.tsx
//=====================================

"use client";

import React, { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "../contexts/AuthContext";
import { useDispatch } from "react-redux";
import { resetState } from "../store/slices/gameSlice";

import styles from "../styles/ModeSelectionPage.module.css";

export type GameMode = "PVE" | "1x1" | "1x2" | "2x2" | "3x3" | "5x5";
type PlayerInfo = { playerId: number; level: number };

type ShopItem = {
    id: number;
    type: "food" | "water";
    name: string;
    description: string;
    image: string;
    effect: Record<string, number>;
    price: number;
};

type PlayerShopState = {
    user_id: number;
    balance: number;
    inventory: string;
};

type ForgeRecipe = {
    id: string;
    name: string;
    description: string;
};

type BaseState = {
    forgeLevel: number;
    built: boolean;
    costs: {
        wood: number;
        stone: number;
        iron: number;
    };
    resources: {
        wood: number;
        stone: number;
        iron: number;
    };
    canBuild: boolean;
    recipes: ForgeRecipe[];
};

// URL сервисов
const API_MATCH =
    process.env.NEXT_PUBLIC_MATCHMAKING_BASE || "http://localhost:8002";
const API_GAME = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8001";
const prepSecondsFromEnv = Number(process.env.NEXT_PUBLIC_PREP_SECONDS || 15);
const PREP_REDIRECT_SECONDS = Number.isFinite(prepSecondsFromEnv)
    ? Math.max(5, Math.min(120, Math.floor(prepSecondsFromEnv)))
    : 15;
const PREP_REDIRECT_MS = PREP_REDIRECT_SECONDS * 1000;

const REQUIRED_SIZE: Record<GameMode, number> = {
    PVE: 1,
    "1x1": 2,
    "1x2": 3,
    "2x2": 4,
    "3x3": 6,
    "5x5": 10,
};

async function fetchQueueSize(mode: GameMode): Promise<number> {
    const res = await fetch(`${API_MATCH}/matchmaking/status?mode=${mode}`);
    if (!res.ok) throw new Error("Не удалось получить размер очереди");
    const queue: any[] = await res.json();
    return queue.length;
}

async function joinQueue(
    mode: GameMode,
    player: PlayerInfo,
    token: string,
): Promise<void> {
    const res = await fetch(`${API_MATCH}/matchmaking/join`, {
        method: "POST",
        headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
            player_id: player.playerId,
            mode,
            rating: player.level * 100,
        }),
    });
    if (!res.ok) {
        throw new Error(
            (await res.text()) || "Ошибка при вступлении в очередь",
        );
    }
}

async function pollCurrentMatch(
    playerId: number,
    token: string,
): Promise<string> {
    while (true) {
        const res = await fetch(
            `${API_MATCH}/matchmaking/currentMatch?player_id=${playerId}`,
            {
                headers: { Authorization: `Bearer ${token}` },
            },
        );
        const data = await res.json();
        if (data.instance_id) {
            return data.instance_id;
        }
        await new Promise((r) => setTimeout(r, 3000));
    }
}

export async function startMatchmaking(
    mode: GameMode,
    player: PlayerInfo,
    token: string,
): Promise<string> {
    await joinQueue(mode, player, token);
    return pollCurrentMatch(player.playerId, token);
}

const MODES: { value: GameMode; label: string }[] = [
    { value: "PVE", label: "PvE (Solo)" },
    { value: "1x1", label: "PvP 1x1" },
    { value: "1x2", label: "PvP 1x2" },
    { value: "2x2", label: "PvP 2x2" },
    { value: "3x3", label: "PvP 3x3" },
    { value: "5x5", label: "PvP 5x5" },
];

export default function ModeSelectionPage() {
    const [mode, setMode] = useState<GameMode>("PVE");
    const [queueSizes, setQueueSizes] = useState<Record<GameMode, number>>({
        PVE: 0,
        "1x1": 0,
        "1x2": 0,
        "2x2": 0,
        "3x3": 0,
        "5x5": 0,
    });
    const router = useRouter();
    const { user, isLoading } = useAuth();
    const [isMatching, setIsMatching] = useState(false);
    const [showShop, setShowShop] = useState(false);
    const [showBase, setShowBase] = useState(false);
    const [pendingInstanceId, setPendingInstanceId] = useState<string | null>(
        null,
    );
    const [redirectAtMs, setRedirectAtMs] = useState<number | null>(null);
    const [shopItems, setShopItems] = useState<ShopItem[]>([]);
    const [shopPlayer, setShopPlayer] = useState<PlayerShopState | null>(null);
    const [shopCounts, setShopCounts] = useState<Record<string, number>>({});
    const [shopBusyType, setShopBusyType] = useState<string | null>(null);
    const [shopError, setShopError] = useState<string>("");
    const [shopInfo, setShopInfo] = useState<string>("");
    const [baseState, setBaseState] = useState<BaseState | null>(null);
    const [baseBusy, setBaseBusy] = useState(false);
    const [baseError, setBaseError] = useState("");
    const [baseInfo, setBaseInfo] = useState("");
    const dispatch = useDispatch();

    const loadShopData = React.useCallback(async () => {
        if (!user) return;
        try {
            const [itemsRes, playerRes] = await Promise.all([
                fetch(`${API_GAME}/game/shop/items`),
                fetch(`${API_GAME}/game/player/${user.id}`),
            ]);

            if (itemsRes.ok) {
                const itemsData = await itemsRes.json();
                const items = itemsData.items || [];
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

    const loadBaseState = React.useCallback(async () => {
        if (!user?.token) return;
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
        } catch (e: any) {
            setBaseError(e?.message || "Не удалось загрузить базу");
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
        } catch (e: any) {
            setBaseError(e?.message || "Ошибка постройки кузницы");
        } finally {
            setBaseBusy(false);
        }
    }, [user]);

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
            } catch (e: any) {
                setShopError(e?.message || "Не удалось купить предмет");
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

    useEffect(() => {
        if (window.location.pathname === "/mode") {
            dispatch(resetState());
        }
    }, [dispatch]);

    useEffect(() => {
        let mounted = true;
        async function updateAll() {
            try {
                const entries = await Promise.all(
                    (Object.keys(REQUIRED_SIZE) as GameMode[]).map(
                        async (m) => {
                            const size = await fetchQueueSize(m);
                            return [m, size] as [GameMode, number];
                        },
                    ),
                );
                if (!mounted) return;
                const sizes = Object.fromEntries(entries) as Record<
                    GameMode,
                    number
                >;
                setQueueSizes(sizes);
            } catch (e) {
                console.error(e);
            }
        }
        updateAll();
        const timer = setInterval(updateAll, 5000);
        return () => {
            mounted = false;
            clearInterval(timer);
        };
    }, []);

    useEffect(() => {
        if (!user) {
            router.push("/login");
        }
    }, [user, router]);

    useEffect(() => {
        loadShopData();
    }, [loadShopData]);

    useEffect(() => {
        loadBaseState();
    }, [loadBaseState]);

    useEffect(() => {
        if (!user) return;
        async function checkQueueStatus() {
            // 1. Проверяем, есть ли уже матч (уже есть эта логика)
            if (!user) return;
            const matchRes = await fetch(
                `${API_MATCH}/matchmaking/currentMatch?player_id=${user.id}`,
                { headers: { Authorization: `Bearer ${user.token}` } },
            );
            const matchData = await matchRes.json();
            if (matchData.instance_id) {
                setPendingInstanceId(matchData.instance_id);
                setRedirectAtMs(Date.now() + PREP_REDIRECT_MS);
                setIsMatching(false);
                return;
            }

            // 2. Проверяем, стоит ли пользователь в очереди
            const queueRes = await fetch(
                `${API_MATCH}/matchmaking/inQueue?player_id=${user.id}`,
            );
            const queueData = await queueRes.json();
            if (queueData.inQueue && queueData.mode) {
                setIsMatching(true);
                setMode(queueData.mode); // чтобы подсветить режим
            } else {
                setIsMatching(false);
            }
        }

        checkQueueStatus();
    }, [user]);

    useEffect(() => {
        if (!isMatching || !user || pendingInstanceId) return;

        let alive = true;
        const timer = setInterval(async () => {
            try {
                const matchRes = await fetch(
                    `${API_MATCH}/matchmaking/currentMatch?player_id=${user.id}`,
                    { headers: { Authorization: `Bearer ${user.token}` } },
                );
                const matchData = await matchRes.json();
                if (!alive) return;

                if (matchData.instance_id) {
                    setPendingInstanceId(matchData.instance_id);
                    setRedirectAtMs(Date.now() + PREP_REDIRECT_MS);
                    setIsMatching(false);
                }
            } catch (e) {
                console.error("poll currentMatch failed", e);
            }
        }, 2000);

        return () => {
            alive = false;
            clearInterval(timer);
        };
    }, [isMatching, user, pendingInstanceId]);

    useEffect(() => {
        if (!pendingInstanceId || !redirectAtMs) return;

        const delay = Math.max(0, redirectAtMs - Date.now());
        const t = setTimeout(() => {
            router.push(`/game?instance_id=${pendingInstanceId}`);
        }, delay);

        return () => clearTimeout(t);
    }, [pendingInstanceId, redirectAtMs, router]);

    const token = user?.token;

    function isTokenExpired(token: string) {
        try {
            // JWT = header.payload.signature
            const [, payload] = token.split(".");
            if (!payload) return true;
            // В браузере atob декодирует Base64
            const decoded = JSON.parse(atob(payload));
            // exp — время в секундах с эпохи Unix
            return decoded.exp * 1000 < Date.now();
        } catch (e) {
            // Если не получилось распарсить — считаем, что токен невалиден/просрочен
            return true;
        }
    }

    async function handleStart() {
        if (!token) {
            alert("Сначала выполните вход");
            router.push("/login");
            return;
        }
        // Проверка по времени
        if (isTokenExpired(token)) {
            alert("Сессия истекла, пожалуйста, войдите снова");
            localStorage.removeItem("user"); // Очистим устаревшие данные
            router.push("/login");
            return;
        }
        try {
            setIsMatching(true);
            setPendingInstanceId(null);
            setRedirectAtMs(null);
            const playerInfo: PlayerInfo = {
                playerId: user!.id,
                level: user!.level,
            };
            // передаём токен
            const instanceId = await startMatchmaking(mode, playerInfo, token);
            setPendingInstanceId(instanceId);
            setRedirectAtMs(Date.now() + PREP_REDIRECT_MS);
        } catch (err) {
            console.error(err);
            setIsMatching(false);
            alert("Ошибка при подборе игроков");
        }
    }

    async function handleCancel() {
        const token = user?.token;
        const playerId = user?.id;
        if (!token || !playerId) {
            alert("Сначала выполните вход");
            router.push("/login");
            return;
        }

        try {
            setIsMatching(false);
            setPendingInstanceId(null);
            setRedirectAtMs(null);

            const res = await fetch(`${API_MATCH}/matchmaking/cancel`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${token}`,
                },
                body: JSON.stringify({
                    player_id: playerId,
                    mode: mode,
                }),
            });

            if (!res.ok) {
                setIsMatching(true);
                const text = await res.text();
                throw new Error(text || "Не удалось выйти из очереди");
            }
        } catch (error) {
            console.error(error);
            alert(`Ошибка при выходе из очереди: ${error}`);
        }
    }

    return (
        <div className={styles.pageRoot}>
            <h2 className={styles.pageTitle}>Выберите режим:</h2>
            <div className={styles.modeContainer}>
                {(Object.keys(REQUIRED_SIZE) as GameMode[]).map((m) => (
                    <div
                        key={m}
                        className={`${styles.modeBlock} ${
                            mode === m ? styles.modeBlockSelected : ""
                        }`}
                        onClick={() => setMode(m)}
                    >
                        <div>{m === "PVE" ? "PvE (Solo)" : `PvP ${m}`}</div>
                        <div
                            style={{ marginTop: "0.5rem", fontSize: "0.9rem" }}
                        >
                            {queueSizes[m]} / {REQUIRED_SIZE[m]}
                        </div>
                    </div>
                ))}
            </div>

            <div className={styles.buttonGroup}>
                <button
                    className={styles.queueButton}
                    onClick={handleStart}
                    disabled={isMatching || !!pendingInstanceId}
                >
                    Вступить в очередь
                </button>
                <button
                    className={styles.queueButton}
                    onClick={handleCancel}
                    disabled={!isMatching || !!pendingInstanceId}
                >
                    Выйти из очереди
                </button>
                <button
                    className={styles.queueButton}
                    onClick={() => setShowShop((v) => !v)}
                >
                    {showShop ? "Закрыть магазин" : "Открыть магазин"}
                </button>
                <button
                    className={styles.queueButton}
                    onClick={() => setShowBase((v) => !v)}
                >
                    {showBase ? "Закрыть базу" : "Открыть базу"}
                </button>
                <button
                    className={styles.queueButton}
                    onClick={() => router.push("/profile")}
                >
                    Профиль
                </button>
            </div>

            {pendingInstanceId && (
                <div className={styles.matchReadyBox}>
                    <div className={styles.matchReadyTitle}>Матч готов</div>
                    <div className={styles.matchReadyText}>
                        Можно докупить предметы. Автостарт через{" "}
                        {Math.max(
                            1,
                            Math.ceil((redirectAtMs! - Date.now()) / 1000),
                        )}{" "}
                        сек.
                    </div>
                    <button
                        className={styles.queueButton}
                        onClick={() =>
                            router.push(
                                `/game?instance_id=${pendingInstanceId}`,
                            )
                        }
                    >
                        Начать матч сейчас
                    </button>
                </div>
            )}

            {showShop && (
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

                    {shopError && (
                        <p className={styles.shopError}>{shopError}</p>
                    )}
                    {shopInfo && <p className={styles.shopInfo}>{shopInfo}</p>}

                    <div className={styles.shopGrid}>
                        {shopItems.map((item) => {
                            const owned = getItemCountFromInventory(item.id);
                            const currentCount = shopCounts[item.type] ?? 1;
                            const totalCost = item.price * currentCount;
                            const canBuy =
                                (shopPlayer?.balance ?? 0) >= totalCost;

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
                                                    increaseShopCount(
                                                        item.type,
                                                        -1,
                                                    )
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
                                                    increaseShopCount(
                                                        item.type,
                                                        1,
                                                    )
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
                                                className={
                                                    styles.shopQuickButton
                                                }
                                                onClick={() =>
                                                    increaseShopCount(
                                                        item.type,
                                                        1,
                                                    )
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
                                                className={
                                                    styles.shopQuickButton
                                                }
                                                onClick={() =>
                                                    increaseShopCount(
                                                        item.type,
                                                        5,
                                                    )
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
                                                className={
                                                    styles.shopQuickButton
                                                }
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
                                            handleBuyShopItem(
                                                item,
                                                currentCount,
                                            )
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
            )}

            {showBase && (
                <section className={styles.basePanel}>
                    <div className={styles.baseHeader}>
                        <h3 className={styles.baseTitle}>База: Кузница</h3>
                        <p className={styles.baseSubtitle}>
                            Кузница строится в лобби и открывает новые рецепты
                            для подготовки к матчам.
                        </p>
                    </div>

                    {baseError && (
                        <p className={styles.baseError}>{baseError}</p>
                    )}
                    {baseInfo && <p className={styles.baseInfo}>{baseInfo}</p>}

                    {baseState ? (
                        <>
                            <div className={styles.baseStatusRow}>
                                <span>
                                    Статус:{" "}
                                    {baseState.built
                                        ? "Построена"
                                        : "Не построена"}
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
                                    {baseBusy
                                        ? "Строим..."
                                        : "Построить кузницу"}
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
            )}
        </div>
    );
}
