//=====================================
// src/components/ModeSelectionPage.tsx
//=====================================

"use client";

import UiText from "@/components/UiText";

import React, { useState, useEffect, useRef } from "react";
import { useRouter } from "next/navigation";
import {
    API_BASE as API_GAME,
    MATCHMAKING_BASE as API_MATCH,
} from "@/utils/serviceUrls";
import { useAuth } from "../contexts/AuthContext";
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
    HeroesState,
    HeroState,
} from "../types";
import StorageLoadout from "./StorageLoadout";
import type { StorageLoadoutHandle } from "./StorageLoadout";
import type { SaveStatus } from "@/features/storage/loadoutModel";
import { availableBattleHeroes, confirmedHeroActivation } from "@/features/heroes/battleHero";
import { getHeroPortrait } from "@/features/heroes/heroArtwork";
import ModeCard from "./ModeCard";
import { Button, StatusMessage } from "./ui";
import { normalizeAvatarPath } from "../utils/normalizeAvatarPath";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";

import styles from "../styles/ModeSelectionPage.module.css";
import modesStyles from "../styles/ModesPage.module.css";
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

const MAX_PARTY_SIZE = 5;

type ActiveHeroPreview = {
    id: string;
    displayName: string;
    description: string;
    image: string;
};

function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === "object" && value !== null;
}

function readString(
    source: Record<string, unknown>,
    key: string,
    fallback = "",
): string {
    const value = source[key];
    return typeof value === "string" ? value : fallback;
}

function titleFromHeroId(heroId: string): string {
    return heroId
        .split(/[-_]/)
        .filter(Boolean)
        .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
        .join(" ");
}

function normalizeActiveHeroPreview(value: unknown): {
    activeHeroClassId: string;
    activeHero: ActiveHeroPreview | null;
} {
    if (!isRecord(value)) {
        return { activeHeroClassId: "", activeHero: null };
    }

    const activeHeroClassId = readString(value, "activeHeroClassId").trim();
    const rawHeroes = Array.isArray(value.heroes) ? value.heroes : [];
    const heroes = rawHeroes
        .filter(isRecord)
        .map((hero) => {
            const id = readString(hero, "id").trim();
            if (!id) return null;
            const displayName =
                readString(hero, "displayName").trim() || titleFromHeroId(id);
            const description = readString(hero, "description").trim();
            const image = readString(hero, "image").trim();
            return {
                id,
                displayName,
                description,
                image,
                active: hero.active === true,
            };
        })
        .filter((hero): hero is ActiveHeroPreview & { active: boolean } =>
            Boolean(hero),
        );

    const activeHero =
        heroes.find((hero) => hero.id === activeHeroClassId) ||
        heroes.find((hero) => hero.active) ||
        null;

    if (activeHero) {
        return {
            activeHeroClassId: activeHeroClassId || activeHero.id,
            activeHero: {
                id: activeHero.id,
                displayName: activeHero.displayName,
                description: activeHero.description,
                image: activeHero.image,
            },
        };
    }

    if (activeHeroClassId) {
        return {
            activeHeroClassId,
            activeHero: {
                id: activeHeroClassId,
                displayName: titleFromHeroId(activeHeroClassId),
                description: "",
                image: "",
            },
        };
    }

    return { activeHeroClassId: "", activeHero: null };
}

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
    if (!res.ok) throw new Error("Failed to get queue size");
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
        throw new Error("Failed to load party");
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
        throw new Error("Failed to load friends");
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
        throw new Error("Failed to load party invites");
    }
    const data: PartyInvitesResponse = await res.json();
    return data.invites || [];
}

async function fetchHeroes(token: string): Promise<HeroesState> {
    const res = await fetch(withBust(`${API_GAME}/game/heroes`), {
        headers: {
            Authorization: `Bearer ${token}`,
        },
        cache: "no-store",
    });
    if (!res.ok) {
        throw new Error((await res.text()) || "Failed to load active hero");
    }
    return res.json();
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
        throw new Error((await res.text()) || "Failed to join queue");
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
    const { language } = useGameLanguage();
    const t = (source: string) => uiText(language, source);
    const [mode, setMode] = useState<GameMode>("PVE");
    useEffect(() => {
        const repeatMode = new URLSearchParams(window.location.search).get("repeatMode");
        if (repeatMode && Object.prototype.hasOwnProperty.call(REQUIRED_SIZE, repeatMode)) setMode(repeatMode as GameMode);
    }, []);
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
    const [loadoutAvailable, setLoadoutAvailable] = useState(false);
    const [startBusy, setStartBusy] = useState(false);
    const [queueError, setQueueError] = useState("");
    const [loadoutStatus, setLoadoutStatus] = useState<SaveStatus>("saving");
    const [loadoutError, setLoadoutError] = useState("");
    const loadoutRef = useRef<StorageLoadoutHandle>(null);
    const startBusyRef = useRef(false);
    const [partyOpen, setPartyOpen] = useState(true);
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
    const [activeHero, setActiveHero] = useState<ActiveHeroPreview | null>(
        null,
    );
    const [activeHeroClassId, setActiveHeroClassId] = useState("");
    const [activeHeroLoading, setActiveHeroLoading] = useState(false);
    const [activeHeroError, setActiveHeroError] = useState("");
    const [heroRoster, setHeroRoster] = useState<HeroState[]>([]);
    const [heroDialogOpen, setHeroDialogOpen] = useState(false);
    const [heroBusyId, setHeroBusyId] = useState<string | null>(null);
    const heroDialogRef = useRef<HTMLDialogElement>(null);
    const heroOpenerRef = useRef<HTMLButtonElement>(null);
    const heroSaveRef = useRef<Promise<void> | null>(null);
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
            setPartyState((prev) =>
                prev
                    ? {
                          ...party,
                          queueMode: prev.queueMode || party.queueMode || "",
                      }
                    : party,
            );
            setPartyFriends(friends);
            setPartyInvites(invites);
        } catch (e: any) {
            setPartyError(e?.message || "Failed to load party");
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
            setPartyState((prev) =>
                prev
                    ? {
                          ...party,
                          queueMode: prev.queueMode || party.queueMode || "",
                      }
                    : party,
            );
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
                throw new Error((await res.text()) || "Failed to update party");
            }
            const nextParty: PartyStateResponse = await res.json();
            setPartyState((prev) =>
                prev
                    ? {
                          ...nextParty,
                          queueMode:
                              prev.queueMode || nextParty.queueMode || "",
                      }
                    : nextParty,
            );
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
                        (await res.text()) || "Failed to send invite",
                    );
                }
                const nextParty: PartyStateResponse = await res.json();
                setPartyState((prev) =>
                    prev
                        ? {
                              ...nextParty,
                              queueMode:
                                  prev.queueMode || nextParty.queueMode || "",
                          }
                        : nextParty,
                );
                const friends = await fetchFriends(user.token);
                setPartyFriends(friends);
                setPartyInfo("Party invite sent");
            } catch (e: any) {
                setPartyError(e?.message || "Failed to send invite");
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
                    (await res.text()) || "Failed to accept invite",
                );
            }
            const party: PartyStateResponse = await res.json();
            setPartyState((prev) =>
                prev
                    ? {
                          ...party,
                          queueMode: prev.queueMode || party.queueMode || "",
                      }
                    : party,
            );
            setPartyInvites([]);
            setPartyInfo("You joined the party");
        } catch (e: any) {
            setPartyError(e?.message || "Failed to accept invite");
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
                    (await res.text()) || "Failed to reject invite",
                );
            }
            setPartyInvites([]);
            setPartyInfo("Invite rejected");
        } catch (e: any) {
            setPartyError(e?.message || "Failed to reject invite");
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
                    "Player removed from party",
                );
            } catch (e: any) {
                setPartyError(e?.message || "Failed to remove player");
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
                "You left the party",
            );
        } catch (e: any) {
            setPartyError(e?.message || "Failed to leave party");
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
                "Party disbanded",
            );
        } catch (e: any) {
            setPartyError(e?.message || "Failed to disband party");
        } finally {
            setPartyBusyUserId(null);
            setPartyBusyAction(null);
        }
    }, [mutateParty, user]);

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
        if (!user?.token) {
            setActiveHero(null);
            setActiveHeroClassId("");
            setHeroRoster([]);
            setActiveHeroError("");
            setActiveHeroLoading(false);
            return;
        }

        let alive = true;
        setActiveHeroLoading(true);
        setActiveHeroError("");
        fetchHeroes(user.token)
            .then((data) => {
                if (!alive) return;
                const preview = normalizeActiveHeroPreview(data);
                setActiveHero(preview.activeHero);
                setActiveHeroClassId(preview.activeHeroClassId);
                setHeroRoster(data.heroes ?? []);
            })
            .catch((error) => {
                if (!alive) return;
                setActiveHero(null);
                setActiveHeroClassId("");
                setHeroRoster([]);
                setActiveHeroError(
                    error instanceof Error
                        ? error.message
                        : "Active hero unavailable",
                );
            })
            .finally(() => {
                if (alive) setActiveHeroLoading(false);
            });

        return () => {
            alive = false;
        };
    }, [user?.token]);

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
                `Mode ${mode} is unavailable for the current party size, switched to ${fallbackMode}`,
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
        return () => {
            /* no-op cleanup: removed frequent polling to avoid extra requests */
        };
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

    // modal is handled globally by GlobalMatchListener

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

    useEffect(() => {
        const dialog = heroDialogRef.current;
        if (!dialog) return;
        if (heroDialogOpen && !dialog.open) dialog.showModal();
        if (!heroDialogOpen && dialog.open) {
            dialog.close();
            heroOpenerRef.current?.focus();
        }
    }, [heroDialogOpen]);

    function changeHero(hero: HeroState) {
        if (!user?.token || heroBusyId || heroSaveRef.current || !hero.owned || hero.locked || !hero.enabled || isMatching || startBusy) return;
        setHeroBusyId(hero.id);
        setActiveHeroError("");
        const request = (async () => {
            const response = await fetch(`${API_GAME}/game/heroes/active`, {
                method: "POST",
                headers: { Authorization: `Bearer ${user.token}`, "Content-Type": "application/json" },
                body: JSON.stringify({ heroClassId: hero.id }),
            });
            if (!response.ok) {
                const body = await response.text();
                let reason = body;
                try { reason = JSON.parse(body).error || body; } catch { /* Plain text response. */ }
                const messages: Record<string, string> = {
                    hero_not_owned: "You do not own this hero.",
                    hero_not_found: "This hero is unavailable.",
                    failed_to_set_active_hero: "The server could not activate this hero.",
                };
                throw new Error(messages[reason] || reason || "Failed to change hero");
            }
            const data: HeroesState = await response.json();
            const preview = normalizeActiveHeroPreview(data);
            if (!Array.isArray(data.heroes) || !confirmedHeroActivation(data, hero.id)) {
                throw new Error("The server did not confirm the selected hero.");
            }
            setActiveHero(preview.activeHero);
            setActiveHeroClassId(preview.activeHeroClassId);
            setHeroRoster(data.heroes);
            setHeroDialogOpen(false);
        })();
        heroSaveRef.current = request;
        void request.catch((error) => {
            setActiveHeroError(error instanceof Error ? error.message : "Failed to change hero");
        }).finally(() => {
            heroSaveRef.current = null;
            setHeroBusyId(null);
        });
    }

    async function handleStart() {
        if (startBusyRef.current) return;
        if (!loadoutAvailable) {
            setQueueError("Loadout unavailable. Wait for it to load before searching.");
            return;
        }
        if (!token) {
            alert("Please log in first");
            router.push("/login");
            return;
        }
        // Проверка по времени
        if (isTokenExpired(token)) {
            alert("Session expired, please log in again");
            localStorage.removeItem("user"); // Clear stale data
            router.push("/login");
            return;
        }
        startBusyRef.current = true;
        setStartBusy(true);
        setQueueError("");
        try {
            if (heroSaveRef.current) await heroSaveRef.current;
            if (activeHeroLoading || !activeHeroClassId) throw new Error("Wait for the active hero to load before searching.");
            if (!loadoutRef.current) throw new Error("Loadout unavailable");
            await loadoutRef.current.ensureSaved();
            const [freshParty, freshInvites] = await Promise.all([
                fetchPartyState(user!.id, token),
                fetchPartyInvites(user!.id, token),
            ]);
            setPartyState((prev) =>
                prev
                    ? {
                          ...freshParty,
                          queueMode:
                              prev.queueMode || freshParty.queueMode || "",
                      }
                    : freshParty,
            );
            setPartyInvites(freshInvites);

            const effectivePartySize = freshParty?.partySize ?? 1;
            const effectiveIsLeader =
                !freshParty?.inParty || freshParty.isLeader;

            if (!effectiveIsLeader) {
                throw new Error("Only the party leader can start matchmaking");
            }
            if (effectivePartySize > TEAM_SIZE[mode]) {
                throw new Error(
                    "Party size exceeds available team size for the selected mode",
                );
            }
            setIsMatching(true);
            setPendingInstanceId(null);
            setRedirectAtMs(null);
            const playerInfo: PlayerInfo = {
                playerId: user!.id,
                level: user!.level,
            };
            // отправляем запрос в очередь и при успешном ответе обновляем UI сразу
            await joinQueue(mode, playerInfo, token);
            startBusyRef.current = false;
            setStartBusy(false);
            setPartyState((prev) =>
                prev
                    ? { ...prev, queueMode: mode }
                    : ({ queueMode: mode } as any),
            );
            setMode(mode);
            // теперь ждём создания матча
            const instanceId = await pollCurrentMatch(
                playerInfo.playerId,
                token,
            );
            setPendingInstanceId(instanceId);
            setRedirectAtMs(Date.now() + PREP_REDIRECT_MS);
        } catch (err) {
            console.error(err);
            setIsMatching(false);
            setQueueError(err instanceof Error ? err.message : "Error during matchmaking");
        } finally {
            startBusyRef.current = false;
            setStartBusy(false);
        }
    }

    async function handleCancel() {
        setQueueError("");
        const token = user?.token;
        const playerId = user?.id;
        if (!token || !playerId) {
            alert("Please log in first");
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
                throw new Error(text || "Failed to leave the queue");
            }
            setPartyState((prev) => (prev ? { ...prev, queueMode: "" } : prev));
        } catch (error) {
            console.error(error);
            setQueueError(`Error leaving the queue: ${error}`);
        }
    }

    function openPlayerProfile(userId: number) {
        if (!userId) return;
        router.push(`/profile?view=${userId}&from=lobby`);
    }

    return (
        <div className={`${styles.pageRoot} ${modesStyles.workspace}`}>
            <header className={modesStyles.heroBanner}>
                <div className={modesStyles.heroCopy}>
                    <span className={modesStyles.eyebrow}><UiText text="THE ROAD AHEAD" /></span>
                    <h2><UiText text="Choose a Mode" /></h2>
                    <p><UiText text="Gather your party, ready your supplies, and choose where the next battle begins." /></p>
                </div>
            </header>
            <section className={modesStyles.battleSetup} aria-label="Battle Setup and matchmaking">
                <div className={modesStyles.setupHeading}>
                    <div><span className={modesStyles.eyebrow}><UiText text="BATTLE SETUP" /></span><h3><UiText text="Ready your next battle" /></h3></div>
                    <div className={modesStyles.selectedMode}><UiText text="Mode" /> <strong>{mode === "PVE" ? "PvE Solo" : `PvP ${mode}`}</strong></div>
                </div>
                <div className={modesStyles.setupRow}>
                    <button ref={heroOpenerRef} type="button" className={modesStyles.heroTrigger} onClick={() => setHeroDialogOpen(true)} disabled={activeHeroLoading || !heroRoster.length || !!heroBusyId || isMatching || startBusy}>
                        {activeHero && <img src={getHeroPortrait(activeHero.id, activeHero.image)} alt="" />}
                        <span className={modesStyles.heroSummary}><span><UiText text="ACTIVE HERO" /></span><strong>{activeHero?.displayName || (activeHeroLoading ? t("Loading hero…") : t("Hero unavailable"))}</strong><small>{activeHeroClassId ? `${t("Class")}: ${activeHeroClassId}` : t("Choose a hero")}</small></span>
                        <span className={modesStyles.editCue}><UiText text="Change ↗" /></span>
                    </button>
                    <StorageLoadout ref={loadoutRef} compact allowSelection collapsible locked={startBusy || isMatching} forceClose={isMatching || !!pendingInstanceId} onAvailabilityChange={setLoadoutAvailable} onStatusChange={(status, error) => { setLoadoutStatus(status); setLoadoutError(error ?? ""); }} />
                    <div className={modesStyles.queueControl}>
                        <div className={modesStyles.queueStatus}><span className={modesStyles.eyebrow}><UiText text="MATCHMAKING" /></span><h3>{t(pendingInstanceId ? "Match found" : isMatching ? "Searching for a match" : startBusy ? "Preparing search" : "Ready to search")}</h3><p><strong>{mode === "PVE" ? "PvE Solo" : `PvP ${mode}`}</strong> · {queueSizes[mode]} <UiText text="in queue" /></p></div>
                        {isMatching && <span className={modesStyles.searchPulse} aria-hidden="true" />}
                        <Button variant={isMatching ? "secondary" : "primary"} className={modesStyles.queueAction} onClick={isMatching ? handleCancel : handleStart} disabled={startBusy || pendingInstanceId !== null || (!isMatching && (activeHeroLoading || !!heroBusyId || !activeHeroClassId || !loadoutAvailable || loadoutStatus === "error" || (!!partyState?.inParty && !partyState.isLeader) || !canPlayMode(mode)))}>{t(pendingInstanceId ? "Match found" : isMatching ? "Cancel search" : startBusy ? "Preparing…" : "Start search")}</Button>
                    </div>
                </div>
                {(queueError || activeHeroError || (loadoutStatus === "error" && loadoutError)) && <div className={modesStyles.setupAlerts}>
                    {queueError && <StatusMessage tone="error">{t(queueError)}</StatusMessage>}
                    {activeHeroError && <StatusMessage tone="error"><UiText text="Hero:" /> {activeHeroError}</StatusMessage>}
                    {loadoutStatus === "error" && <StatusMessage tone="error"><UiText text="Loadout:" /> {loadoutError} <button type="button" onClick={() => { void loadoutRef.current?.retry().catch(() => undefined); }}><UiText text="Retry loadout" /></button></StatusMessage>}
                </div>}
            </section>
            <div className={modesStyles.mainGrid}>
            <aside className={modesStyles.sidebar} aria-label="Party and friends">
            <section className={`${styles.partyPanel} ${modesStyles.partyPanel}`}>
                <div className={styles.partyHeaderRow}>
                    <div>
                        <h3 className={styles.partyTitle}><UiText text="Party" /></h3>
                        <p className={styles.partySubtitle}>
                            <UiText text="The leader queues the whole group. Party members will be placed on the same team." />
                        </p>
                    </div>
                    <div className={styles.partyMeta}>
                        {partyState?.partySize ?? 1} / {MAX_PARTY_SIZE} <UiText text="players" />
                        {partyState?.queueMode
                            ? ` • queue ${partyState.queueMode}`
                            : ""}
                    </div>
                    <button type="button" className={modesStyles.partyToggle} aria-expanded={partyOpen} aria-controls="mode-party-body" onClick={() => setPartyOpen(open => !open)}>{t(partyOpen ? "Hide" : "Show")} <UiText text="party" /></button>
                </div>
                <div id="mode-party-body" className={modesStyles.partyBody} data-open={partyOpen}>
                {partyError && (
                    <p className={styles.partyError}>{partyError}</p>
                )}
                {partyInfo && <p className={styles.partyInfo}>{partyInfo}</p>}
                {partyLoading && (
                    <p className={styles.partyMuted}><UiText text="Loading party..." /></p>
                )}

                {!partyState?.inParty && partyInvites.length > 0 && (
                    <div className={styles.partyInviteSection}>
                        <h4 className={styles.partyInviteTitle}>
                            <UiText text="Incoming party invite" />
                        </h4>
                        {partyInvites.map((invite) => (
                            <article
                                key={`${invite.leader.user_id}-${invite.createdAt}`}
                                className={`${styles.partyInviteCard} ${modesStyles.personRow}`}
                            >
                                <img src={normalizeAvatarPath(invite.leader.image)} alt="" className={styles.partyAvatar} />
                                <div className={styles.partyInviteMeta}>
                                    <strong>{invite.leader.name}</strong>
                                    <span>
                                        <UiText text="Class:" /> {invite.leader.characterType}
                                    </span>
                                    <span><UiText text="Level:" /> {invite.leader.level}</span>
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
                                        <UiText text="Profile" />
                                    </button>
                                    <button
                                        type="button"
                                        className={styles.partyActionButton}
                                        onClick={handleAcceptInvite}
                                        disabled={
                                            partyBusyAction === "acceptInvite"
                                        }
                                    >
                                        <UiText text="Accept" />
                                    </button>
                                    <button
                                        type="button"
                                        className={styles.partySecondaryButton}
                                        onClick={handleRejectInvite}
                                        disabled={
                                            partyBusyAction === "rejectInvite"
                                        }
                                    >
                                        <UiText text="Reject" />
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
                                className={`${styles.partyMemberCard} ${modesStyles.personRow}`}
                            >
                                <img
                                    src={normalizeAvatarPath(member.image)}
                                    alt={member.name}
                                    className={styles.partyAvatar}
                                />
                                <div className={styles.partyMemberMeta}>
                                    <strong>{member.name}</strong>
                                    <span>
                                        {member.characterType || "guardian"}
                                    </span>
                                    <span><UiText text="Level" /> {member.level}</span>
                                    {member.user_id ===
                                        partyState?.leaderId && (
                                        <span
                                            className={styles.partyLeaderBadge}
                                        >
                                            <UiText text="Leader" />
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
                                    <UiText text="Profile" />
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
                                        <UiText text="Remove" />
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
                                <UiText text="Disband party" />
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
                            <UiText text="Leave party" />
                        </button>
                    )}
                </div>
                {partyState?.isLeader && !isMatching && (
                    <div className={styles.partyInviteSection}>
                        <h4 className={styles.partyInviteTitle}>
                            <UiText text="Friends for party" />
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
                                        className={`${styles.partyInviteCard} ${modesStyles.personRow}`}
                                    >
                                        <img src={normalizeAvatarPath(friend.image)} alt="" className={styles.partyAvatar} />
                                        <div className={styles.partyInviteMeta}>
                                            <strong>{friend.name}</strong>
                                            <span>{friend.characterType}</span>
                                            <span><UiText text="Level" /> {friend.level}</span>
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
                                            <UiText text="Profile" />
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
                                                (partyState.partySize ?? 1) >=
                                                    MAX_PARTY_SIZE
                                            }
                                        >
                                            <UiText text="Invite" />
                                        </button>
                                    </article>
                                ))}
                        </div>
                        {partyFriends.length === 0 && (
                            <p className={styles.partyMuted}>
                                <UiText text="Add friends in your profile to build parties." />
                            </p>
                        )}
                    </div>
                )}
                </div>
            </section>
            </aside>
            <section className={modesStyles.modeSection} aria-labelledby="mode-section-title">
                <div className={modesStyles.sectionHeading}><div><span className={modesStyles.eyebrow}><UiText text="BATTLEFIELDS" /></span><h3 id="mode-section-title"><UiText text="Select your path" /></h3></div><span>{currentPartySize} <UiText text="in your party" /></span></div>
                <div className={modesStyles.modeGrid}>
                    {(Object.keys(REQUIRED_SIZE) as GameMode[]).map((m) => <ModeCard key={m} mode={m} selected={mode === m} disabled={!canPlayMode(m)} matching={isMatching && partyState?.queueMode === m} queuedPlayers={queueSizes[m]} requiredPlayers={REQUIRED_SIZE[m]} partySize={currentPartySize} onSelect={() => setMode(m)} />)}
                </div>
            </section>
            </div>

            <dialog ref={heroDialogRef} className={modesStyles.heroDialog} aria-labelledby="hero-dialog-title" onCancel={(event) => { event.preventDefault(); if (!heroBusyId) setHeroDialogOpen(false); }}>
                <div className={modesStyles.heroDialogHeader}><div><span className={modesStyles.eyebrow}><UiText text="BATTLE SETUP" /></span><h2 id="hero-dialog-title"><UiText text="Choose your hero" /></h2><p><UiText text="Available heroes for this account" /></p></div><button type="button" onClick={() => setHeroDialogOpen(false)} disabled={!!heroBusyId} aria-label="Close hero selection">×</button></div>
                <div className={modesStyles.heroChoices}>
                    {availableBattleHeroes(heroRoster).map(hero => <button type="button" key={hero.id} className={modesStyles.heroChoice} aria-current={hero.id === activeHeroClassId ? "true" : undefined} disabled={!!heroBusyId || isMatching || startBusy || hero.id === activeHeroClassId} onClick={() => changeHero(hero)}>
                        <img src={getHeroPortrait(hero.id, hero.image)} alt="" />
                        <span><strong>{hero.displayName}</strong><small><UiText text="Class:" /> {hero.id}</small><small>{hero.description}</small></span>
                        <em>{t(heroBusyId === hero.id ? "Activating…" : hero.id === activeHeroClassId ? "Active" : "Select")}</em>
                    </button>)}
                    {availableBattleHeroes(heroRoster).length === 0 && <p><UiText text="No available heroes." /></p>}
                </div>
                {activeHeroError && <p className={modesStyles.heroDialogError} role="alert">{activeHeroError}</p>}
            </dialog>

            {/* Modal moved to global listener mounted in layout */}
        </div>
    );
}
