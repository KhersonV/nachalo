//=====================================
// src/components/ModeSelectionPage.tsx
//=====================================

"use client";

import React, { useState, useEffect, useRef } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "../contexts/AuthContext";
import { useDispatch } from "react-redux";
import { resetState } from "../store/slices/gameSlice";
import type {
    FriendSummary,
    GameMode,
    PartyInviteState,
    PartyInvitesResponse,
    PartyStateResponse,
    CurrentMatchResponse,
    InQueueResponse,
    PlayerInfo,
    QueueSizeResponse,
} from "../types";
import LobbyHeader from "./LobbyHeader";
import { normalizeAvatarPath } from "../utils/normalizeAvatarPath";

import styles from "../styles/ModeSelectionPage.module.css";

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
    const data: QueueSizeResponse = await res.json();
    if (!Array.isArray(data) && typeof data?.totalPlayers === "number") {
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
        const data: CurrentMatchResponse = await res.json();
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
    const { user } = useAuth();
    const [isMatching, setIsMatching] = useState(false);
    const [pendingInstanceId, setPendingInstanceId] = useState<string | null>(
        null,
    );
    const [redirectAtMs, setRedirectAtMs] = useState<number | null>(null);
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
            const matchData: CurrentMatchResponse = await matchRes.json();
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
            const queueData: InQueueResponse = await queueRes.json();
            const knownModes = Object.keys(REQUIRED_SIZE) as GameMode[];
            if (
                queueData.inQueue &&
                queueData.mode &&
                knownModes.includes(queueData.mode as GameMode)
            ) {
                const queueMode = queueData.mode as GameMode;
                setIsMatching(true);
                setMode(queueMode); // чтобы подсветить режим
                setPartyState((prev) =>
                    prev ? { ...prev, queueMode: String(queueMode) } : prev,
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
                const matchData: CurrentMatchResponse = await matchRes.json();
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

    function openPlayerProfile(userId: number) {
        if (!userId) return;
        router.push(`/profile?view=${userId}&from=lobby`);
    }

    return (
        <div className={styles.pageRoot}>
            <LobbyHeader />
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
                                        className={styles.partySecondaryButton}
                                        onClick={() =>
                                            openPlayerProfile(
                                                invite.leader.user_id,
                                            )
                                        }
                                    >
                                        Профиль
                                    </button>
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
                                    src={normalizeAvatarPath(member.image)}
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
                                <button
                                    type="button"
                                    className={styles.partySecondaryButton}
                                    onClick={() =>
                                        openPlayerProfile(member.user_id)
                                    }
                                >
                                    Профиль
                                </button>
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
                                            className={
                                                styles.partySecondaryButton
                                            }
                                            onClick={() =>
                                                openPlayerProfile(friend.userId)
                                            }
                                            disabled={
                                                partyBusyUserId ===
                                                friend.userId
                                            }
                                        >
                                            Профиль
                                        </button>
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
                                className={`${styles.modeBlock} 
                                    ${mode === m ? styles.modeBlockSelected : ""}
                                    ${!modeAllowed ? styles.modeBlockDisabled : ""}
                                    ${
                                        isMatching &&
                                        String(partyState?.queueMode) ===
                                            String(m)
                                            ? mode === m
                                                ? styles.modeBlockSelectedMatching
                                                : styles.modeBlockMatching
                                            : ""
                                    }`}
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
        </div>
    );
}
