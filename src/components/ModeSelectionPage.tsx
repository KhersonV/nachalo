//=====================================
// src/components/ModeSelectionPage.tsx
//=====================================

"use client";

import React, { useState, useEffect, useRef } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "../contexts/AuthContext";
import { useDispatch } from "react-redux";
import { resetState } from "../store/slices/gameSlice";

import styles from "../styles/ModeSelectionPage.module.css";

export type GameMode = "PVE" | "1x1" | "1x2" | "2x2" | "3x3" | "5x5";
type PlayerInfo = { playerId: number; level: number };

type FriendSummary = {
    userId: number;
    name: string;
    image: string;
    characterType: string;
    level: number;
    activityStatus: "in_match" | "in_lobby" | "offline";
};

type PartyMemberState = {
    user_id: number;
    name: string;
    image: string;
    characterType: string;
    level: number;
};

type PartyStateResponse = {
    inParty: boolean;
    partyId?: string;
    leaderId: number;
    isLeader: boolean;
    members: PartyMemberState[];
    partySize: number;
    queueMode?: string;
};

type PartyInviteState = {
    leader: PartyMemberState;
    partyId?: string;
    createdAt: string;
};

type PartyInvitesResponse = {
    status: string;
    invites: PartyInviteState[];
};

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

const TEAM_SIZE: Record<GameMode, number> = {
    PVE: 1,
    "1x1": 1,
    "1x2": 1,
    "2x2": 2,
    "3x3": 3,
    "5x5": 5,
};

function withBust(url: string): string {
    const sep = url.includes("?") ? "&" : "?";
    return `${url}${sep}t=${Date.now()}`;
}

async function fetchQueueSize(mode: GameMode): Promise<number> {
    const res = await fetch(
        withBust(`${API_MATCH}/matchmaking/status?mode=${mode}`),
        {
            cache: "no-store",
        },
    );
    if (!res.ok) throw new Error("Не удалось получить размер очереди");
    const data = await res.json();
    if (typeof data?.totalPlayers === "number") {
        return data.totalPlayers;
    }
    if (Array.isArray(data)) {
        return data.length;
    }
    return 0;
}

async function fetchPartyState(
    playerId: number,
    token: string,
): Promise<PartyStateResponse> {
    const res = await fetch(
        withBust(`${API_MATCH}/matchmaking/party?player_id=${playerId}`),
        {
            headers: {
                Authorization: `Bearer ${token}`,
            },
            cache: "no-store",
        },
    );
    if (!res.ok) {
        throw new Error("Не удалось загрузить пати");
    }
    return res.json();
}

async function fetchFriends(token: string): Promise<FriendSummary[]> {
    const res = await fetch(withBust(`${API_GAME}/game/friends`), {
        headers: {
            Authorization: `Bearer ${token}`,
        },
        cache: "no-store",
    });
    if (!res.ok) {
        throw new Error("Не удалось загрузить друзей");
    }
    const data = await res.json();
    return data.data || [];
}

async function fetchPartyInvites(
    playerId: number,
    token: string,
): Promise<PartyInviteState[]> {
    const res = await fetch(
        withBust(
            `${API_MATCH}/matchmaking/party/invites?player_id=${playerId}`,
        ),
        {
            headers: {
                Authorization: `Bearer ${token}`,
            },
            cache: "no-store",
        },
    );
    if (!res.ok) {
        throw new Error("Не удалось загрузить инвайты в пати");
    }
    const data: PartyInvitesResponse = await res.json();
    return data.invites || [];
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
            withBust(
                `${API_MATCH}/matchmaking/currentMatch?player_id=${playerId}`,
            ),
            {
                headers: {
                    Authorization: `Bearer ${token}`,
                },
                cache: "no-store",
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
    const [partyState, setPartyState] = useState<PartyStateResponse | null>(
        null,
    );
    const [partyFriends, setPartyFriends] = useState<FriendSummary[]>([]);
    const [partyLoading, setPartyLoading] = useState(false);
    const [partyError, setPartyError] = useState("");
    const [partyInfo, setPartyInfo] = useState("");
    const [partyBusyUserId, setPartyBusyUserId] = useState<number | null>(null);
    const [partyBusyAction, setPartyBusyAction] = useState<string | null>(null);
    const [partyInvites, setPartyInvites] = useState<PartyInviteState[]>([]);
    const dispatch = useDispatch();
    const currentPartySize = partyState?.partySize ?? 1;
    const partyRefreshInFlightRef = useRef(false);

    const canPlayMode = React.useCallback(
        (targetMode: GameMode) => currentPartySize <= TEAM_SIZE[targetMode],
        [currentPartySize],
    );

    const loadPartyData = React.useCallback(async () => {
        if (!user?.token) return;
        setPartyLoading(true);
        setPartyError("");
        try {
            const [party, friends, invites] = await Promise.all([
                fetchPartyState(user.id, user.token),
                fetchFriends(user.token),
                fetchPartyInvites(user.id, user.token),
            ]);
            setPartyState(party);
            setPartyFriends(friends);
            setPartyInvites(invites);
        } catch (e: any) {
            setPartyError(e?.message || "Не удалось загрузить пати");
        } finally {
            setPartyLoading(false);
        }
    }, [user]);

    const refreshPartyDataSilently = React.useCallback(async () => {
        if (!user?.token) return;
        if (partyRefreshInFlightRef.current) return;
        partyRefreshInFlightRef.current = true;
        try {
            const [party, invites] = await Promise.all([
                fetchPartyState(user.id, user.token),
                fetchPartyInvites(user.id, user.token),
            ]);
            setPartyState(party);
            setPartyInvites(invites);
        } catch {
            // Silent refresh keeps current UI state on transient network errors.
        } finally {
            partyRefreshInFlightRef.current = false;
        }
    }, [user]);

    const mutateParty = React.useCallback(
        async (
            path: string,
            body: Record<string, number>,
            successMessage?: string,
        ) => {
            if (!user?.token) return;
            const res = await fetch(`${API_MATCH}${path}`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${user.token}`,
                },
                body: JSON.stringify(body),
            });
            if (!res.ok) {
                throw new Error(
                    (await res.text()) || "Не удалось обновить пати",
                );
            }
            const nextParty: PartyStateResponse = await res.json();
            setPartyState(nextParty);
            if (successMessage) {
                setPartyInfo(successMessage);
            }
            const friends = await fetchFriends(user.token);
            setPartyFriends(friends);
        },
        [user],
    );

    const handleAddToParty = React.useCallback(
        async (memberId: number) => {
            if (!user) return;
            setPartyError("");
            setPartyInfo("");
            setPartyBusyUserId(memberId);
            setPartyBusyAction("invite");
            try {
                const res = await fetch(
                    `${API_MATCH}/matchmaking/party/invite`,
                    {
                        method: "POST",
                        headers: {
                            "Content-Type": "application/json",
                            Authorization: `Bearer ${user.token}`,
                        },
                        body: JSON.stringify({
                            leader_id: user.id,
                            member_id: memberId,
                        }),
                    },
                );
                if (!res.ok) {
                    throw new Error(
                        (await res.text()) || "Не удалось отправить инвайт",
                    );
                }
                setPartyInfo("Инвайт в пати отправлен");
            } catch (e: any) {
                setPartyError(e?.message || "Не удалось отправить инвайт");
            } finally {
                setPartyBusyUserId(null);
                setPartyBusyAction(null);
            }
        },
        [user],
    );

    const handleAcceptInvite = React.useCallback(async () => {
        if (!user) return;
        setPartyError("");
        setPartyInfo("");
        setPartyBusyUserId(user.id);
        setPartyBusyAction("acceptInvite");
        try {
            const res = await fetch(
                `${API_MATCH}/matchmaking/party/invite/accept`,
                {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        Authorization: `Bearer ${user.token}`,
                    },
                    body: JSON.stringify({ player_id: user.id }),
                },
            );
            if (!res.ok) {
                throw new Error(
                    (await res.text()) || "Не удалось принять инвайт",
                );
            }
            const party: PartyStateResponse = await res.json();
            setPartyState(party);
            setPartyInvites([]);
            setPartyInfo("Вы вступили в пати");
        } catch (e: any) {
            setPartyError(e?.message || "Не удалось принять инвайт");
        } finally {
            setPartyBusyUserId(null);
            setPartyBusyAction(null);
        }
    }, [user]);

    const handleRejectInvite = React.useCallback(async () => {
        if (!user) return;
        setPartyError("");
        setPartyInfo("");
        setPartyBusyUserId(user.id);
        setPartyBusyAction("rejectInvite");
        try {
            const res = await fetch(
                `${API_MATCH}/matchmaking/party/invite/reject`,
                {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        Authorization: `Bearer ${user.token}`,
                    },
                    body: JSON.stringify({ player_id: user.id }),
                },
            );
            if (!res.ok) {
                throw new Error(
                    (await res.text()) || "Не удалось отклонить инвайт",
                );
            }
            setPartyInvites([]);
            setPartyInfo("Инвайт отклонен");
        } catch (e: any) {
            setPartyError(e?.message || "Не удалось отклонить инвайт");
        } finally {
            setPartyBusyUserId(null);
            setPartyBusyAction(null);
        }
    }, [user]);

    const handleRemoveFromParty = React.useCallback(
        async (memberId: number) => {
            if (!user) return;
            setPartyError("");
            setPartyInfo("");
            setPartyBusyUserId(memberId);
            setPartyBusyAction("remove");
            try {
                await mutateParty(
                    "/matchmaking/party/remove",
                    { leader_id: user.id, member_id: memberId },
                    "Игрок исключён из пати",
                );
            } catch (e: any) {
                setPartyError(e?.message || "Не удалось исключить игрока");
            } finally {
                setPartyBusyUserId(null);
                setPartyBusyAction(null);
            }
        },
        [mutateParty, user],
    );

    const handleLeaveParty = React.useCallback(async () => {
        if (!user) return;
        setPartyError("");
        setPartyInfo("");
        setPartyBusyUserId(user.id);
        setPartyBusyAction("leave");
        try {
            await mutateParty(
                "/matchmaking/party/leave",
                { player_id: user.id },
                "Вы вышли из пати",
            );
        } catch (e: any) {
            setPartyError(e?.message || "Не удалось выйти из пати");
        } finally {
            setPartyBusyUserId(null);
            setPartyBusyAction(null);
        }
    }, [mutateParty, user]);

    const handleDisbandParty = React.useCallback(async () => {
        if (!user) return;
        setPartyError("");
        setPartyInfo("");
        setPartyBusyUserId(user.id);
        setPartyBusyAction("disband");
        try {
            await mutateParty(
                "/matchmaking/party/disband",
                { leader_id: user.id },
                "Пати распущена",
            );
        } catch (e: any) {
            setPartyError(e?.message || "Не удалось распустить пати");
        } finally {
            setPartyBusyUserId(null);
            setPartyBusyAction(null);
        }
    }, [mutateParty, user]);

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
        loadPartyData();
    }, [loadPartyData]);

    useEffect(() => {
        if (!user?.token) return;

        let alive = true;
        const refreshNow = async () => {
            if (!alive) return;
            await refreshPartyDataSilently();
        };

        const timer = setInterval(refreshNow, 1000);
        const onFocus = () => {
            refreshNow();
        };
        const onVisibilityChange = () => {
            if (document.visibilityState === "visible") {
                refreshNow();
            }
        };

        window.addEventListener("focus", onFocus);
        document.addEventListener("visibilitychange", onVisibilityChange);

        return () => {
            alive = false;
            clearInterval(timer);
            window.removeEventListener("focus", onFocus);
            document.removeEventListener(
                "visibilitychange",
                onVisibilityChange,
            );
        };
    }, [user, refreshPartyDataSilently]);

    useEffect(() => {
        if (isMatching) return;
        if (canPlayMode(mode)) return;

        const fallbackMode = (Object.keys(REQUIRED_SIZE) as GameMode[]).find(
            (candidate) => canPlayMode(candidate),
        );
        if (fallbackMode && fallbackMode !== mode) {
            setMode(fallbackMode);
            setPartyInfo(
                `Режим ${mode} недоступен для текущего размера пати, выбран ${fallbackMode}`,
            );
        }
    }, [mode, canPlayMode, isMatching]);

    useEffect(() => {
        if (!user) return;
        async function checkQueueStatus() {
            // 1. Проверяем, есть ли уже матч (уже есть эта логика)
            if (!user) return;
            const matchRes = await fetch(
                withBust(
                    `${API_MATCH}/matchmaking/currentMatch?player_id=${user.id}`,
                ),
                {
                    headers: {
                        Authorization: `Bearer ${user.token}`,
                    },
                    cache: "no-store",
                },
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
                withBust(
                    `${API_MATCH}/matchmaking/inQueue?player_id=${user.id}`,
                ),
                {
                    cache: "no-store",
                },
            );
            const queueData = await queueRes.json();
            if (queueData.inQueue && queueData.mode) {
                setIsMatching(true);
                setMode(queueData.mode); // чтобы подсветить режим
                setPartyState((prev) =>
                    prev
                        ? { ...prev, queueMode: String(queueData.mode) }
                        : prev,
                );
            } else {
                setIsMatching(false);
                setPartyState((prev) =>
                    prev ? { ...prev, queueMode: "" } : prev,
                );
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
                    withBust(
                        `${API_MATCH}/matchmaking/currentMatch?player_id=${user.id}`,
                    ),
                    {
                        headers: {
                            Authorization: `Bearer ${user.token}`,
                        },
                        cache: "no-store",
                    },
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
            const [freshParty, freshInvites] = await Promise.all([
                fetchPartyState(user!.id, token),
                fetchPartyInvites(user!.id, token),
            ]);
            setPartyState(freshParty);
            setPartyInvites(freshInvites);

            const effectivePartySize = freshParty?.partySize ?? 1;
            const effectiveIsLeader =
                !freshParty?.inParty || freshParty.isLeader;

            if (!effectiveIsLeader) {
                throw new Error(
                    "Только лидер пати может запустить поиск матча",
                );
            }
            if (effectivePartySize > TEAM_SIZE[mode]) {
                throw new Error(
                    "Размер пати больше доступной команды для выбранного режима",
                );
            }
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
            setPartyState((prev) =>
                prev ? { ...prev, queueMode: mode } : prev,
            );
        } catch (err) {
            console.error(err);
            setIsMatching(false);
            alert(
                err instanceof Error
                    ? err.message
                    : "Ошибка при подборе игроков",
            );
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
            setPartyState((prev) => (prev ? { ...prev, queueMode: "" } : prev));
        } catch (error) {
            console.error(error);
            alert(`Ошибка при выходе из очереди: ${error}`);
        }
    }

    return (
        <div className={styles.pageRoot}>
            <h2 className={styles.pageTitle}>Выберите режим:</h2>
            <section className={styles.partyPanel}>
                <div className={styles.partyHeaderRow}>
                    <div>
                        <h3 className={styles.partyTitle}>Пати</h3>
                        <p className={styles.partySubtitle}>
                            Лидер ставит в очередь всю группу. Участники пати
                            попадут в одну команду.
                        </p>
                    </div>
                    <div className={styles.partyMeta}>
                        {partyState?.partySize ?? 1} игрок(а)
                        {partyState?.queueMode
                            ? ` • очередь ${partyState.queueMode}`
                            : ""}
                    </div>
                </div>
                {partyError && (
                    <p className={styles.partyError}>{partyError}</p>
                )}
                {partyInfo && <p className={styles.partyInfo}>{partyInfo}</p>}
                {partyLoading && (
                    <p className={styles.partyMuted}>Загрузка пати...</p>
                )}

                {!partyState?.inParty && partyInvites.length > 0 && (
                    <div className={styles.partyInviteSection}>
                        <h4 className={styles.partyInviteTitle}>
                            Входящий инвайт в пати
                        </h4>
                        {partyInvites.map((invite) => (
                            <article
                                key={`${invite.leader.user_id}-${invite.createdAt}`}
                                className={styles.partyInviteCard}
                            >
                                <div className={styles.partyInviteMeta}>
                                    <strong>{invite.leader.name}</strong>
                                    <span>
                                        Класс: {invite.leader.characterType}
                                    </span>
                                    <span>Уровень: {invite.leader.level}</span>
                                </div>
                                <div className={styles.partyActionsRow}>
                                    <button
                                        type="button"
                                        className={styles.partyActionButton}
                                        onClick={handleAcceptInvite}
                                        disabled={
                                            partyBusyAction === "acceptInvite"
                                        }
                                    >
                                        Принять
                                    </button>
                                    <button
                                        type="button"
                                        className={styles.partySecondaryButton}
                                        onClick={handleRejectInvite}
                                        disabled={
                                            partyBusyAction === "rejectInvite"
                                        }
                                    >
                                        Отклонить
                                    </button>
                                </div>
                            </article>
                        ))}
                    </div>
                )}

                <div className={styles.partyMembers}>
                    {(partyState?.members || []).map((member) => {
                        const isSelf = member.user_id === user?.id;
                        const canKick =
                            partyState?.isLeader && !isSelf && !isMatching;
                        const busy = partyBusyUserId === member.user_id;
                        return (
                            <article
                                key={member.user_id}
                                className={styles.partyMemberCard}
                            >
                                <img
                                    src={member.image || "/player-1.webp"}
                                    alt={member.name}
                                    className={styles.partyAvatar}
                                />
                                <div className={styles.partyMemberMeta}>
                                    <strong>{member.name}</strong>
                                    <span>
                                        {member.characterType || "adventurer"}
                                    </span>
                                    <span>Уровень {member.level}</span>
                                    {member.user_id ===
                                        partyState?.leaderId && (
                                        <span
                                            className={styles.partyLeaderBadge}
                                        >
                                            Лидер
                                        </span>
                                    )}
                                </div>
                                {canKick && (
                                    <button
                                        type="button"
                                        className={styles.partyActionButton}
                                        onClick={() =>
                                            handleRemoveFromParty(
                                                member.user_id,
                                            )
                                        }
                                        disabled={busy}
                                    >
                                        Исключить
                                    </button>
                                )}
                            </article>
                        );
                    })}
                </div>
                <div className={styles.partyActionsRow}>
                    {partyState?.inParty &&
                        partyState.isLeader &&
                        partyState.partySize > 1 && (
                            <button
                                type="button"
                                className={styles.partySecondaryButton}
                                onClick={handleDisbandParty}
                                disabled={
                                    !!isMatching ||
                                    partyBusyAction === "disband"
                                }
                            >
                                Распустить пати
                            </button>
                        )}
                    {partyState?.inParty && !partyState.isLeader && (
                        <button
                            type="button"
                            className={styles.partySecondaryButton}
                            onClick={handleLeaveParty}
                            disabled={
                                !!isMatching || partyBusyAction === "leave"
                            }
                        >
                            Выйти из пати
                        </button>
                    )}
                </div>
                {partyState?.isLeader && !isMatching && (
                    <div className={styles.partyInviteSection}>
                        <h4 className={styles.partyInviteTitle}>
                            Друзья для пати
                        </h4>
                        <div className={styles.partyInviteGrid}>
                            {partyFriends
                                .filter(
                                    (friend) =>
                                        !(partyState.members || []).some(
                                            (member) =>
                                                member.user_id ===
                                                friend.userId,
                                        ),
                                )
                                .map((friend) => (
                                    <article
                                        key={friend.userId}
                                        className={styles.partyInviteCard}
                                    >
                                        <div className={styles.partyInviteMeta}>
                                            <strong>{friend.name}</strong>
                                            <span>{friend.characterType}</span>
                                            <span>Уровень {friend.level}</span>
                                            <span>{friend.activityStatus}</span>
                                        </div>
                                        <button
                                            type="button"
                                            className={styles.partyActionButton}
                                            onClick={() =>
                                                handleAddToParty(friend.userId)
                                            }
                                            disabled={
                                                partyBusyUserId ===
                                                    friend.userId ||
                                                (partyState.partySize ?? 1) >= 5
                                            }
                                        >
                                            Пригласить
                                        </button>
                                    </article>
                                ))}
                        </div>
                        {partyFriends.length === 0 && (
                            <p className={styles.partyMuted}>
                                Добавь друзей в профиле, чтобы собирать пати.
                            </p>
                        )}
                    </div>
                )}
            </section>
            <div className={styles.modeContainer}>
                {(Object.keys(REQUIRED_SIZE) as GameMode[]).map((m) =>
                    (() => {
                        const modeAllowed = canPlayMode(m);
                        return (
                            <div
                                key={m}
                                className={`${styles.modeBlock} ${
                                    mode === m ? styles.modeBlockSelected : ""
                                } ${!modeAllowed ? styles.modeBlockDisabled : ""}`}
                                onClick={() => {
                                    if (modeAllowed) {
                                        setMode(m);
                                    }
                                }}
                            >
                                <div>
                                    {m === "PVE" ? "PvE (Solo)" : `PvP ${m}`}
                                </div>
                                <div
                                    style={{
                                        marginTop: "0.5rem",
                                        fontSize: "0.9rem",
                                    }}
                                >
                                    {queueSizes[m]} / {REQUIRED_SIZE[m]}
                                </div>
                                {!modeAllowed && (
                                    <div
                                        style={{
                                            marginTop: "0.3rem",
                                            fontSize: "0.8rem",
                                        }}
                                    >
                                        Недоступно для пати из{" "}
                                        {currentPartySize}
                                    </div>
                                )}
                            </div>
                        );
                    })(),
                )}
            </div>

            <div className={styles.buttonGroup}>
                <button
                    className={styles.queueButton}
                    onClick={handleStart}
                    disabled={
                        isMatching ||
                        !!pendingInstanceId ||
                        (!!partyState?.inParty && !partyState.isLeader) ||
                        !canPlayMode(mode)
                    }
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
