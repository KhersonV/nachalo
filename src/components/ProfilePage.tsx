"use client";

import React, { useCallback, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "../contexts/AuthContext";
import LobbyHeader from "./LobbyHeader";
import { normalizeAvatarPath } from "../utils/normalizeAvatarPath";
import styles from "../styles/ProfilePage.module.css";

type ProfileApiResponse = {
    status: string;
    data: {
        player: {
            userId: number;
            name: string;
            image: string;
            online: boolean;
            activityStatus: "in_match" | "in_lobby" | "offline";
            characterType: string;
            level: number;
            experience: number;
            maxExperience: number;
            balance: number;
            attack: number;
            defense: number;
            mobility: number;
            agility: number;
            sightRange: number;
            isRanged: boolean;
            attackRange: number;
        };
        progress: {
            matchesPlayed: number;
            wins: number;
            winRate: number;
            totalExpGained: number;
            playerKills: number;
            monsterKills: number;
            damageTotal: number;
        };
        resources: {
            food: number;
            water: number;
            wood: number;
            stone: number;
            iron: number;
        };
        base: {
            forgeLevel: number;
            built: boolean;
            canBuild: boolean;
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
            recipes: Array<{
                id: string;
                name: string;
                description: string;
            }>;
        };
    };
};

type PublicProfileApiResponse = {
    status: string;
    data: {
        player: ProfileApiResponse["data"]["player"];
        progress: ProfileApiResponse["data"]["progress"];
        isFriend: boolean;
        friendRelation: "self" | "friend" | "outgoing" | "incoming" | "none";
    };
};

type FriendsApiResponse = {
    status: string;
    data: Array<{
        userId: number;
        name: string;
        image: string;
        characterType: string;
        level: number;
        activityStatus: "in_match" | "in_lobby" | "offline";
    }>;
};

type SearchPlayersApiResponse = {
    status: string;
    data: Array<{
        userId: number;
        name: string;
        image: string;
        characterType: string;
        level: number;
        isFriend: boolean;
        friendRelation: "self" | "friend" | "outgoing" | "incoming" | "none";
        activityStatus: "in_match" | "in_lobby" | "offline";
    }>;
};

type IncomingFriendRequestsApiResponse = {
    status: string;
    data: Array<{
        userId: number;
        name: string;
        image: string;
        characterType: string;
        level: number;
        activityStatus: "in_match" | "in_lobby" | "offline";
        createdAt: string;
    }>;
};

type OutgoingFriendRequestsApiResponse = {
    status: string;
    data: Array<{
        userId: number;
        name: string;
        image: string;
        characterType: string;
        level: number;
        activityStatus: "in_match" | "in_lobby" | "offline";
        createdAt: string;
    }>;
};

const API_GAME = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8001";

export default function ProfilePage() {
    const router = useRouter();
    const searchParams = useSearchParams();
    const { user } = useAuth();

    const [profile, setProfile] = useState<ProfileApiResponse["data"] | null>(
        null,
    );
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");

    const [editName, setEditName] = useState("");
    const [editImage, setEditImage] = useState("");
    const [saving, setSaving] = useState(false);
    const [info, setInfo] = useState("");

    const [friends, setFriends] = useState<FriendsApiResponse["data"]>([]);
    const [friendsError, setFriendsError] = useState("");
    const [incomingRequests, setIncomingRequests] = useState<
        IncomingFriendRequestsApiResponse["data"]
    >([]);
    const [requestsError, setRequestsError] = useState("");
    const [outgoingRequests, setOutgoingRequests] = useState<
        OutgoingFriendRequestsApiResponse["data"]
    >([]);
    const [outgoingRequestsError, setOutgoingRequestsError] = useState("");

    const [lookupId, setLookupId] = useState("");
    const [viewedProfile, setViewedProfile] = useState<
        PublicProfileApiResponse["data"] | null
    >(null);
    const [lookupLoading, setLookupLoading] = useState(false);
    const [lookupError, setLookupError] = useState("");
    const [addingFriend, setAddingFriend] = useState(false);
    const [removingFriendUserId, setRemovingFriendUserId] = useState<
        number | null
    >(null);
    const [processingRequestUserId, setProcessingRequestUserId] = useState<
        number | null
    >(null);
    const [cancelingOutgoingUserId, setCancelingOutgoingUserId] = useState<
        number | null
    >(null);
    const [friendInfo, setFriendInfo] = useState("");

    const [searchQuery, setSearchQuery] = useState("");
    const [searchingPlayers, setSearchingPlayers] = useState(false);
    const [searchResults, setSearchResults] = useState<
        SearchPlayersApiResponse["data"]
    >([]);

    const fetchProfile = useCallback(async () => {
        if (!user?.token) return null;
        const res = await fetch(`${API_GAME}/game/profile`, {
            headers: {
                Authorization: `Bearer ${user.token}`,
            },
            cache: "no-store",
        });
        if (!res.ok) {
            throw new Error("Не удалось загрузить профиль");
        }
        const data: ProfileApiResponse = await res.json();
        return data.data;
    }, [user]);

    const loadProfile = useCallback(async () => {
        if (!user?.token) return;

        setLoading(true);
        setError("");
        try {
            const data = await fetchProfile();
            if (!data) return;
            setProfile(data);
            setEditName(data.player.name || "");
            setEditImage(normalizeAvatarPath(data.player.image));
        } catch (e: any) {
            setError(e?.message || "Ошибка загрузки профиля");
        } finally {
            setLoading(false);
        }
    }, [user, fetchProfile]);

    const fetchFriends = useCallback(async () => {
        if (!user?.token) return [];
        const res = await fetch(`${API_GAME}/game/friends`, {
            headers: {
                Authorization: `Bearer ${user.token}`,
            },
            cache: "no-store",
        });
        if (!res.ok) {
            throw new Error("Не удалось загрузить друзей");
        }
        const data: FriendsApiResponse = await res.json();
        return data.data;
    }, [user]);

    const fetchIncomingRequests = useCallback(async () => {
        if (!user?.token) return [];
        const res = await fetch(`${API_GAME}/game/friends/requests/incoming`, {
            headers: {
                Authorization: `Bearer ${user.token}`,
            },
            cache: "no-store",
        });
        if (!res.ok) {
            throw new Error("Не удалось загрузить входящие заявки");
        }
        const data: IncomingFriendRequestsApiResponse = await res.json();
        return data.data;
    }, [user]);

    const fetchOutgoingRequests = useCallback(async () => {
        if (!user?.token) return [];
        const res = await fetch(`${API_GAME}/game/friends/requests/outgoing`, {
            headers: {
                Authorization: `Bearer ${user.token}`,
            },
            cache: "no-store",
        });
        if (!res.ok) {
            throw new Error("Не удалось загрузить исходящие заявки");
        }
        const data: OutgoingFriendRequestsApiResponse = await res.json();
        return data.data;
    }, [user]);

    useEffect(() => {
        loadProfile();
    }, [loadProfile]);

    useEffect(() => {
        if (!user?.token) return;

        let alive = true;
        (async () => {
            try {
                const data = await fetchFriends();
                if (!alive) return;
                setFriends(data);
                setFriendsError("");
            } catch (e: any) {
                if (!alive) return;
                setFriendsError(e?.message || "Ошибка загрузки друзей");
            }
        })();

        return () => {
            alive = false;
        };
    }, [user, fetchFriends]);

    useEffect(() => {
        if (!user?.token) return;

        let alive = true;
        (async () => {
            try {
                const data = await fetchIncomingRequests();
                if (!alive) return;
                setIncomingRequests(data);
                setRequestsError("");
            } catch (e: any) {
                if (!alive) return;
                setRequestsError(e?.message || "Ошибка загрузки заявок");
            }
        })();

        return () => {
            alive = false;
        };
    }, [user, fetchIncomingRequests]);

    useEffect(() => {
        if (!user?.token) return;

        let alive = true;
        (async () => {
            try {
                const data = await fetchOutgoingRequests();
                if (!alive) return;
                setOutgoingRequests(data);
                setOutgoingRequestsError("");
            } catch (e: any) {
                if (!alive) return;
                setOutgoingRequestsError(
                    e?.message || "Ошибка загрузки исходящих заявок",
                );
            }
        })();

        return () => {
            alive = false;
        };
    }, [user, fetchOutgoingRequests]);

    useEffect(() => {
        if (!user?.token) return;

        let alive = true;

        const refreshSilently = async () => {
            try {
                const data = await fetchProfile();
                if (!alive || !data) return;
                setProfile(data);

                const friendsData = await fetchFriends();
                if (!alive) return;
                setFriends(friendsData);

                const incomingData = await fetchIncomingRequests();
                if (!alive) return;
                setIncomingRequests(incomingData);

                const outgoingData = await fetchOutgoingRequests();
                if (!alive) return;
                setOutgoingRequests(outgoingData);
            } catch {
                // Silent polling: keep existing UI state if refresh fails.
            }
        };

        const interval = window.setInterval(refreshSilently, 8000);

        const onVisibilityChange = () => {
            if (!document.hidden) {
                refreshSilently();
            }
        };
        document.addEventListener("visibilitychange", onVisibilityChange);

        return () => {
            alive = false;
            window.clearInterval(interval);
            document.removeEventListener(
                "visibilitychange",
                onVisibilityChange,
            );
        };
    }, [
        user,
        fetchProfile,
        fetchFriends,
        fetchIncomingRequests,
        fetchOutgoingRequests,
    ]);

    const loadPublicProfileById = useCallback(
        async (userId: number) => {
            if (!user?.token) return;

            if (!Number.isFinite(userId) || userId <= 0) {
                setLookupError("Введите корректный ID игрока");
                return;
            }

            setLookupLoading(true);
            setLookupError("");
            setFriendInfo("");

            try {
                const res = await fetch(`${API_GAME}/game/profile/${userId}`, {
                    headers: {
                        Authorization: `Bearer ${user.token}`,
                    },
                    cache: "no-store",
                });
                if (!res.ok) {
                    const text = await res.text();
                    throw new Error(
                        text || "Не удалось загрузить профиль игрока",
                    );
                }
                const data: PublicProfileApiResponse = await res.json();
                setViewedProfile(data.data);
            } catch (e: any) {
                setViewedProfile(null);
                setLookupError(e?.message || "Ошибка загрузки профиля игрока");
            } finally {
                setLookupLoading(false);
            }
        },
        [user],
    );

    const viewOtherProfile = useCallback(async () => {
        const parsed = Number.parseInt(lookupId.trim(), 10);
        if (!Number.isFinite(parsed) || parsed <= 0) {
            setLookupError("Введите корректный ID игрока");
            return;
        }

        await loadPublicProfileById(parsed);
    }, [lookupId, loadPublicProfileById]);

    const openFriendProfile = useCallback(
        async (friendUserID: number) => {
            setLookupId(String(friendUserID));
            await loadPublicProfileById(friendUserID);
        },
        [loadPublicProfileById],
    );

    useEffect(() => {
        if (!user?.token) return;

        const viewParam = searchParams.get("view");
        if (!viewParam) return;

        const parsedId = Number.parseInt(viewParam, 10);
        if (!Number.isFinite(parsedId) || parsedId <= 0) return;

        setLookupId(String(parsedId));
        void loadPublicProfileById(parsedId);
    }, [user, searchParams, loadPublicProfileById]);

    const addFriend = useCallback(async () => {
        if (!user?.token || !viewedProfile) return;
        if (viewedProfile.player.userId === user.id) {
            setLookupError("Нельзя добавить себя в друзья");
            return;
        }

        setAddingFriend(true);
        setLookupError("");
        setFriendInfo("");
        try {
            const res = await fetch(`${API_GAME}/game/friends/add`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${user.token}`,
                },
                body: JSON.stringify({
                    friendUserId: viewedProfile.player.userId,
                }),
            });
            if (!res.ok) {
                const text = await res.text();
                throw new Error(text || "Не удалось добавить в друзья");
            }

            setViewedProfile((prev) =>
                prev
                    ? {
                          ...prev,
                          isFriend: false,
                          friendRelation: "outgoing",
                      }
                    : prev,
            );
            const friendsData = await fetchFriends();
            setFriends(friendsData);
            const incomingData = await fetchIncomingRequests();
            setIncomingRequests(incomingData);
            setSearchResults((prev) =>
                prev.map((item) =>
                    item.userId === viewedProfile.player.userId
                        ? {
                              ...item,
                              friendRelation: "outgoing",
                              isFriend: false,
                          }
                        : item,
                ),
            );
            setFriendInfo("Заявка в друзья отправлена");
        } catch (e: any) {
            setLookupError(e?.message || "Ошибка добавления в друзья");
        } finally {
            setAddingFriend(false);
        }
    }, [user, viewedProfile, fetchFriends, fetchIncomingRequests]);

    const removeFriend = useCallback(
        async (friendUserId: number) => {
            if (!user?.token) return;

            setRemovingFriendUserId(friendUserId);
            setLookupError("");
            setFriendInfo("");

            try {
                const res = await fetch(
                    `${API_GAME}/game/friends/${friendUserId}`,
                    {
                        method: "DELETE",
                        headers: {
                            Authorization: `Bearer ${user.token}`,
                        },
                    },
                );
                if (!res.ok) {
                    const text = await res.text();
                    throw new Error(text || "Не удалось удалить из друзей");
                }

                const friendsData = await fetchFriends();
                setFriends(friendsData);
                setSearchResults((prev) =>
                    prev.map((item) =>
                        item.userId === friendUserId
                            ? {
                                  ...item,
                                  isFriend: false,
                                  friendRelation: "none",
                              }
                            : item,
                    ),
                );
                setViewedProfile((prev) =>
                    prev && prev.player.userId === friendUserId
                        ? { ...prev, isFriend: false, friendRelation: "none" }
                        : prev,
                );
                setFriendInfo("Игрок удален из друзей");
            } catch (e: any) {
                setLookupError(e?.message || "Ошибка удаления из друзей");
            } finally {
                setRemovingFriendUserId(null);
            }
        },
        [user, fetchFriends],
    );

    const searchPlayers = useCallback(async () => {
        if (!user?.token) return;

        const q = searchQuery.trim();
        if (q.length < 2) {
            setLookupError("Введите минимум 2 символа для поиска");
            return;
        }

        setSearchingPlayers(true);
        setLookupError("");

        try {
            const res = await fetch(
                `${API_GAME}/game/players/search?q=${encodeURIComponent(q)}`,
                {
                    headers: {
                        Authorization: `Bearer ${user.token}`,
                    },
                    cache: "no-store",
                },
            );
            if (!res.ok) {
                const text = await res.text();
                throw new Error(text || "Не удалось выполнить поиск");
            }

            const data: SearchPlayersApiResponse = await res.json();
            setSearchResults(data.data);
            if (data.data.length === 0) {
                setFriendInfo("По вашему запросу игроки не найдены");
            }
        } catch (e: any) {
            setLookupError(e?.message || "Ошибка поиска игроков");
        } finally {
            setSearchingPlayers(false);
        }
    }, [user, searchQuery]);

    const acceptFriendRequest = useCallback(
        async (requesterUserId: number) => {
            if (!user?.token) return;

            setProcessingRequestUserId(requesterUserId);
            setLookupError("");
            setFriendInfo("");
            try {
                const res = await fetch(
                    `${API_GAME}/game/friends/requests/${requesterUserId}/accept`,
                    {
                        method: "POST",
                        headers: {
                            Authorization: `Bearer ${user.token}`,
                        },
                    },
                );
                if (!res.ok) {
                    const text = await res.text();
                    throw new Error(text || "Не удалось принять заявку");
                }

                const [friendsData, incomingData] = await Promise.all([
                    fetchFriends(),
                    fetchIncomingRequests(),
                ]);
                setFriends(friendsData);
                setIncomingRequests(incomingData);
                setSearchResults((prev) =>
                    prev.map((item) =>
                        item.userId === requesterUserId
                            ? {
                                  ...item,
                                  isFriend: true,
                                  friendRelation: "friend",
                              }
                            : item,
                    ),
                );
                setViewedProfile((prev) =>
                    prev && prev.player.userId === requesterUserId
                        ? { ...prev, isFriend: true, friendRelation: "friend" }
                        : prev,
                );
                setFriendInfo("Заявка принята");
            } catch (e: any) {
                setLookupError(e?.message || "Ошибка принятия заявки");
            } finally {
                setProcessingRequestUserId(null);
            }
        },
        [user, fetchFriends, fetchIncomingRequests],
    );

    const rejectFriendRequest = useCallback(
        async (requesterUserId: number) => {
            if (!user?.token) return;

            setProcessingRequestUserId(requesterUserId);
            setLookupError("");
            setFriendInfo("");
            try {
                const res = await fetch(
                    `${API_GAME}/game/friends/requests/${requesterUserId}/reject`,
                    {
                        method: "POST",
                        headers: {
                            Authorization: `Bearer ${user.token}`,
                        },
                    },
                );
                if (!res.ok) {
                    const text = await res.text();
                    throw new Error(text || "Не удалось отклонить заявку");
                }

                const incomingData = await fetchIncomingRequests();
                setIncomingRequests(incomingData);
                setSearchResults((prev) =>
                    prev.map((item) =>
                        item.userId === requesterUserId
                            ? {
                                  ...item,
                                  isFriend: false,
                                  friendRelation: "none",
                              }
                            : item,
                    ),
                );
                setViewedProfile((prev) =>
                    prev && prev.player.userId === requesterUserId
                        ? { ...prev, isFriend: false, friendRelation: "none" }
                        : prev,
                );
                setFriendInfo("Заявка отклонена");
            } catch (e: any) {
                setLookupError(e?.message || "Ошибка отклонения заявки");
            } finally {
                setProcessingRequestUserId(null);
            }
        },
        [user, fetchIncomingRequests],
    );

    const cancelOutgoingRequest = useCallback(
        async (targetUserId: number) => {
            if (!user?.token) return;

            setCancelingOutgoingUserId(targetUserId);
            setLookupError("");
            setFriendInfo("");
            try {
                const res = await fetch(
                    `${API_GAME}/game/friends/requests/${targetUserId}`,
                    {
                        method: "DELETE",
                        headers: {
                            Authorization: `Bearer ${user.token}`,
                        },
                    },
                );
                if (!res.ok) {
                    const text = await res.text();
                    throw new Error(text || "Не удалось отменить заявку");
                }

                const outgoingData = await fetchOutgoingRequests();
                setOutgoingRequests(outgoingData);
                setSearchResults((prev) =>
                    prev.map((item) =>
                        item.userId === targetUserId
                            ? {
                                  ...item,
                                  isFriend: false,
                                  friendRelation: "none",
                              }
                            : item,
                    ),
                );
                setViewedProfile((prev) =>
                    prev && prev.player.userId === targetUserId
                        ? { ...prev, isFriend: false, friendRelation: "none" }
                        : prev,
                );
                setFriendInfo("Заявка отменена");
            } catch (e: any) {
                setLookupError(e?.message || "Ошибка отмены заявки");
            } finally {
                setCancelingOutgoingUserId(null);
            }
        },
        [user, fetchOutgoingRequests],
    );

    const saveProfile = useCallback(async () => {
        if (!user?.token) return;
        setSaving(true);
        setError("");
        setInfo("");
        try {
            const res = await fetch(`${API_GAME}/game/profile`, {
                method: "PATCH",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${user.token}`,
                },
                body: JSON.stringify({
                    name: editName,
                    image: editImage,
                }),
            });
            if (!res.ok) {
                const text = await res.text();
                throw new Error(text || "Не удалось сохранить профиль");
            }
            const data: ProfileApiResponse = await res.json();
            setProfile(data.data);
            setInfo("Профиль сохранен");
        } catch (e: any) {
            setError(e?.message || "Ошибка сохранения профиля");
        } finally {
            setSaving(false);
        }
    }, [user, editName, editImage]);

    if (loading) {
        return <div className={styles.page}>Загрузка профиля...</div>;
    }

    if (error && !profile) {
        return <div className={styles.page}>Ошибка: {error}</div>;
    }

    if (!profile) {
        return <div className={styles.page}>Профиль недоступен</div>;
    }

    const viewedUserParamRaw = searchParams.get("view");
    const viewedUserParam = viewedUserParamRaw
        ? Number.parseInt(viewedUserParamRaw, 10)
        : NaN;
    const fromParam = searchParams.get("from") || "";
    const returnInstanceId = searchParams.get("instance_id") || "";
    const isExternalProfileMode =
        Number.isFinite(viewedUserParam) && viewedUserParam > 0;

    const activePlayer =
        isExternalProfileMode && viewedProfile
            ? viewedProfile.player
            : profile.player;
    const activeProgress =
        isExternalProfileMode && viewedProfile
            ? viewedProfile.progress
            : profile.progress;

    const expPercent = activePlayer.maxExperience
        ? Math.min(
              100,
              Math.round(
                  (activePlayer.experience / activePlayer.maxExperience) * 100,
              ),
          )
        : 0;

    const getActivityLabel = (status: "in_match" | "in_lobby" | "offline") =>
        status === "in_match"
            ? "В игре"
            : status === "in_lobby"
              ? "В лобби"
              : "Офлайн";

    const getActivityClass = (status: "in_match" | "in_lobby" | "offline") =>
        status === "in_match"
            ? styles.statusInMatch
            : status === "in_lobby"
              ? styles.statusInLobby
              : styles.statusOffline;

    const activityLabel = getActivityLabel(activePlayer.activityStatus);
    const activityClass = getActivityClass(activePlayer.activityStatus);
    const isViewingFromRoute = !!searchParams.get("view");

    const handleBack = () => {
        if (isExternalProfileMode && fromParam === "game" && returnInstanceId) {
            router.push(
                `/game?instance_id=${encodeURIComponent(returnInstanceId)}`,
            );
            return;
        }
        router.push("/mode");
    };

    return (
        <div className={styles.page}>
            {!isExternalProfileMode && (
                <div style={{ marginBottom: "0.95rem" }}>
                    <LobbyHeader />
                </div>
            )}
            <div className={styles.topRow}>
                <h2 className={styles.title}>Профиль игрока</h2>
                <button className={styles.backBtn} onClick={handleBack}>
                    {isExternalProfileMode && fromParam === "game"
                        ? "Назад в игру"
                        : "К режимам"}
                </button>
            </div>

            {isViewingFromRoute && (
                <div className={styles.info}>
                    {viewedProfile
                        ? `Просмотр профиля: ${viewedProfile.player.name}`
                        : "Загружаем профиль игрока..."}
                </div>
            )}

            <section className={styles.card}>
                <div className={styles.identityRow}>
                    <img
                        className={styles.avatar}
                        src={normalizeAvatarPath(activePlayer.image)}
                        alt="avatar"
                    />
                    <div className={styles.identityMeta}>
                        <div className={styles.name}>{activePlayer.name}</div>
                        <div
                            className={`${styles.statusBadge} ${activityClass}`}
                        >
                            {activityLabel}
                        </div>
                        <div className={styles.subline}>
                            Класс: {activePlayer.characterType}
                        </div>
                        <div className={styles.subline}>
                            Баланс: {activePlayer.balance}
                        </div>
                    </div>
                </div>

                <div className={styles.expWrap}>
                    <div className={styles.expLabel}>
                        Уровень {activePlayer.level} • {activePlayer.experience}
                        /{activePlayer.maxExperience} XP
                    </div>
                    <div className={styles.expBarTrack}>
                        <div
                            className={styles.expBarFill}
                            style={{ width: `${expPercent}%` }}
                        />
                    </div>
                </div>

                {!isExternalProfileMode && (
                    <>
                        <div className={styles.editGrid}>
                            <label className={styles.label}>
                                Ник
                                <input
                                    className={styles.input}
                                    value={editName}
                                    onChange={(e) =>
                                        setEditName(e.target.value)
                                    }
                                    maxLength={32}
                                />
                            </label>
                            <label className={styles.label}>
                                URL аватара
                                <input
                                    className={styles.input}
                                    value={editImage}
                                    onChange={(e) =>
                                        setEditImage(e.target.value)
                                    }
                                    maxLength={512}
                                />
                            </label>
                        </div>

                        <div className={styles.inlineActions}>
                            <button
                                className={styles.primaryBtn}
                                onClick={saveProfile}
                                disabled={saving}
                            >
                                {saving ? "Сохраняем..." : "Сохранить"}
                            </button>
                            {info && (
                                <span className={styles.info}>{info}</span>
                            )}
                            {error && (
                                <span className={styles.error}>{error}</span>
                            )}
                        </div>
                    </>
                )}
            </section>

            {!isExternalProfileMode && (
                <section className={styles.card}>
                    <h3 className={styles.sectionTitle}>
                        Профили других игроков
                    </h3>
                    <div className={styles.lookupRow}>
                        <input
                            className={styles.input}
                            value={lookupId}
                            onChange={(e) => setLookupId(e.target.value)}
                            placeholder="Введите ID игрока"
                            inputMode="numeric"
                        />
                        <button
                            className={styles.primaryBtn}
                            onClick={viewOtherProfile}
                            disabled={lookupLoading}
                        >
                            {lookupLoading ? "Ищем..." : "Открыть профиль"}
                        </button>
                    </div>
                    <div className={styles.lookupRow}>
                        <input
                            className={styles.input}
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            placeholder="Поиск игроков по нику"
                        />
                        <button
                            className={styles.primaryBtn}
                            onClick={searchPlayers}
                            disabled={searchingPlayers}
                        >
                            {searchingPlayers ? "Ищем..." : "Найти"}
                        </button>
                    </div>
                    {lookupError && (
                        <div className={styles.error}>{lookupError}</div>
                    )}
                    {friendInfo && (
                        <div className={styles.info}>{friendInfo}</div>
                    )}

                    {searchResults.length > 0 && (
                        <div className={styles.friendList}>
                            {searchResults.map((p) => (
                                <div
                                    key={`search-${p.userId}`}
                                    className={styles.friendItem}
                                >
                                    <img
                                        className={styles.friendAvatar}
                                        src={normalizeAvatarPath(p.image)}
                                        alt="search avatar"
                                    />
                                    <div className={styles.friendMeta}>
                                        <strong>{p.name}</strong>
                                        <span>
                                            ID: {p.userId} • Ур. {p.level} •{" "}
                                            {p.characterType}
                                        </span>
                                    </div>
                                    <div
                                        className={`${styles.statusBadge} ${getActivityClass(p.activityStatus)}`}
                                    >
                                        {getActivityLabel(p.activityStatus)}
                                    </div>
                                    <button
                                        className={styles.backBtn}
                                        onClick={() =>
                                            openFriendProfile(p.userId)
                                        }
                                    >
                                        Открыть
                                    </button>
                                </div>
                            ))}
                        </div>
                    )}

                    {viewedProfile && (
                        <div className={styles.previewCard}>
                            <div className={styles.identityRow}>
                                <img
                                    className={styles.avatar}
                                    src={normalizeAvatarPath(
                                        viewedProfile.player.image,
                                    )}
                                    alt="viewed avatar"
                                />
                                <div className={styles.identityMeta}>
                                    <div className={styles.name}>
                                        {viewedProfile.player.name}
                                    </div>
                                    <div
                                        className={`${styles.statusBadge} ${getActivityClass(viewedProfile.player.activityStatus)}`}
                                    >
                                        {getActivityLabel(
                                            viewedProfile.player.activityStatus,
                                        )}
                                    </div>
                                    <div className={styles.subline}>
                                        ID: {viewedProfile.player.userId} •
                                        Класс:{" "}
                                        {viewedProfile.player.characterType}
                                    </div>
                                    <div className={styles.subline}>
                                        Уровень: {viewedProfile.player.level} •
                                        Winrate:{" "}
                                        {viewedProfile.progress.winRate}%
                                    </div>
                                </div>
                            </div>

                            {viewedProfile.player.userId !== user?.id && (
                                <div className={styles.inlineActions}>
                                    <button
                                        className={styles.primaryBtn}
                                        onClick={addFriend}
                                        disabled={
                                            addingFriend ||
                                            viewedProfile.friendRelation !==
                                                "none"
                                        }
                                    >
                                        {viewedProfile.friendRelation ===
                                        "friend"
                                            ? "Уже в друзьях"
                                            : viewedProfile.friendRelation ===
                                                "outgoing"
                                              ? "Заявка отправлена"
                                              : viewedProfile.friendRelation ===
                                                  "incoming"
                                                ? "Входящая заявка"
                                                : addingFriend
                                                  ? "Отправляем..."
                                                  : "Отправить заявку"}
                                    </button>
                                    {viewedProfile.friendRelation ===
                                        "friend" && (
                                        <button
                                            className={styles.dangerBtn}
                                            onClick={() =>
                                                removeFriend(
                                                    viewedProfile.player.userId,
                                                )
                                            }
                                            disabled={
                                                removingFriendUserId ===
                                                viewedProfile.player.userId
                                            }
                                        >
                                            {removingFriendUserId ===
                                            viewedProfile.player.userId
                                                ? "Удаляем..."
                                                : "Удалить из друзей"}
                                        </button>
                                    )}
                                    {viewedProfile.friendRelation ===
                                        "incoming" && (
                                        <>
                                            <button
                                                className={styles.primaryBtn}
                                                onClick={() =>
                                                    acceptFriendRequest(
                                                        viewedProfile.player
                                                            .userId,
                                                    )
                                                }
                                                disabled={
                                                    processingRequestUserId ===
                                                    viewedProfile.player.userId
                                                }
                                            >
                                                {processingRequestUserId ===
                                                viewedProfile.player.userId
                                                    ? "Обработка..."
                                                    : "Принять заявку"}
                                            </button>
                                            <button
                                                className={styles.dangerBtn}
                                                onClick={() =>
                                                    rejectFriendRequest(
                                                        viewedProfile.player
                                                            .userId,
                                                    )
                                                }
                                                disabled={
                                                    processingRequestUserId ===
                                                    viewedProfile.player.userId
                                                }
                                            >
                                                Отклонить заявку
                                            </button>
                                        </>
                                    )}
                                    {viewedProfile.friendRelation ===
                                        "outgoing" && (
                                        <button
                                            className={styles.dangerBtn}
                                            onClick={() =>
                                                cancelOutgoingRequest(
                                                    viewedProfile.player.userId,
                                                )
                                            }
                                            disabled={
                                                cancelingOutgoingUserId ===
                                                viewedProfile.player.userId
                                            }
                                        >
                                            {cancelingOutgoingUserId ===
                                            viewedProfile.player.userId
                                                ? "Отменяем..."
                                                : "Отменить заявку"}
                                        </button>
                                    )}
                                </div>
                            )}
                        </div>
                    )}
                </section>
            )}

            {!isExternalProfileMode && (
                <section className={styles.card}>
                    <h3 className={styles.sectionTitle}>
                        Входящие заявки в друзья
                    </h3>
                    {requestsError && (
                        <div className={styles.error}>{requestsError}</div>
                    )}
                    {incomingRequests.length === 0 ? (
                        <div className={styles.subline}>
                            Нет входящих заявок
                        </div>
                    ) : (
                        <div className={styles.friendList}>
                            {incomingRequests.map((req) => (
                                <div
                                    key={`request-${req.userId}`}
                                    className={styles.friendItem}
                                >
                                    <img
                                        className={styles.friendAvatar}
                                        src={normalizeAvatarPath(req.image)}
                                        alt="request avatar"
                                    />
                                    <div className={styles.friendMeta}>
                                        <strong>{req.name}</strong>
                                        <span>
                                            ID: {req.userId} • Ур. {req.level} •{" "}
                                            {req.characterType}
                                        </span>
                                    </div>
                                    <div
                                        className={`${styles.statusBadge} ${getActivityClass(req.activityStatus)}`}
                                    >
                                        {getActivityLabel(req.activityStatus)}
                                    </div>
                                    <button
                                        className={styles.primaryBtn}
                                        onClick={() =>
                                            acceptFriendRequest(req.userId)
                                        }
                                        disabled={
                                            processingRequestUserId ===
                                            req.userId
                                        }
                                    >
                                        {processingRequestUserId === req.userId
                                            ? "Обработка..."
                                            : "Принять"}
                                    </button>
                                    <button
                                        className={styles.dangerBtn}
                                        onClick={() =>
                                            rejectFriendRequest(req.userId)
                                        }
                                        disabled={
                                            processingRequestUserId ===
                                            req.userId
                                        }
                                    >
                                        Отклонить
                                    </button>
                                </div>
                            ))}
                        </div>
                    )}
                </section>
            )}

            {!isExternalProfileMode && (
                <section className={styles.card}>
                    <h3 className={styles.sectionTitle}>
                        Исходящие заявки в друзья
                    </h3>
                    {outgoingRequestsError && (
                        <div className={styles.error}>
                            {outgoingRequestsError}
                        </div>
                    )}
                    {outgoingRequests.length === 0 ? (
                        <div className={styles.subline}>
                            Нет исходящих заявок
                        </div>
                    ) : (
                        <div className={styles.friendList}>
                            {outgoingRequests.map((req) => (
                                <div
                                    key={`outgoing-${req.userId}`}
                                    className={styles.friendItem}
                                >
                                    <img
                                        className={styles.friendAvatar}
                                        src={normalizeAvatarPath(req.image)}
                                        alt="outgoing request avatar"
                                    />
                                    <div className={styles.friendMeta}>
                                        <strong>{req.name}</strong>
                                        <span>
                                            ID: {req.userId} • Ур. {req.level} •{" "}
                                            {req.characterType}
                                        </span>
                                    </div>
                                    <div
                                        className={`${styles.statusBadge} ${getActivityClass(req.activityStatus)}`}
                                    >
                                        {getActivityLabel(req.activityStatus)}
                                    </div>
                                    <button
                                        className={styles.backBtn}
                                        onClick={() =>
                                            openFriendProfile(req.userId)
                                        }
                                    >
                                        Открыть
                                    </button>
                                    <button
                                        className={styles.dangerBtn}
                                        onClick={() =>
                                            cancelOutgoingRequest(req.userId)
                                        }
                                        disabled={
                                            cancelingOutgoingUserId ===
                                            req.userId
                                        }
                                    >
                                        {cancelingOutgoingUserId === req.userId
                                            ? "Отменяем..."
                                            : "Отменить"}
                                    </button>
                                </div>
                            ))}
                        </div>
                    )}
                </section>
            )}

            {!isExternalProfileMode && (
                <section className={styles.card}>
                    <h3 className={styles.sectionTitle}>Друзья</h3>
                    {friendsError && (
                        <div className={styles.error}>{friendsError}</div>
                    )}
                    {friends.length === 0 ? (
                        <div className={styles.subline}>Список друзей пуст</div>
                    ) : (
                        <div className={styles.friendList}>
                            {friends.map((f) => (
                                <div
                                    key={f.userId}
                                    className={styles.friendItem}
                                >
                                    <img
                                        className={styles.friendAvatar}
                                        src={normalizeAvatarPath(f.image)}
                                        alt="friend avatar"
                                    />
                                    <div className={styles.friendMeta}>
                                        <strong>{f.name}</strong>
                                        <span>
                                            ID: {f.userId} • Ур. {f.level} •{" "}
                                            {f.characterType}
                                        </span>
                                    </div>
                                    <div
                                        className={`${styles.statusBadge} ${getActivityClass(f.activityStatus)}`}
                                    >
                                        {getActivityLabel(f.activityStatus)}
                                    </div>
                                    <button
                                        className={styles.backBtn}
                                        onClick={() =>
                                            openFriendProfile(f.userId)
                                        }
                                    >
                                        Быстрый просмотр
                                    </button>
                                    <button
                                        className={styles.dangerBtn}
                                        onClick={() => removeFriend(f.userId)}
                                        disabled={
                                            removingFriendUserId === f.userId
                                        }
                                    >
                                        {removingFriendUserId === f.userId
                                            ? "Удаляем..."
                                            : "Удалить"}
                                    </button>
                                </div>
                            ))}
                        </div>
                    )}
                </section>
            )}

            <section className={styles.grid}>
                <article className={styles.card}>
                    <h3 className={styles.sectionTitle}>Прогресс</h3>
                    <div className={styles.statRow}>
                        <span>Матчей</span>
                        <strong>{activeProgress.matchesPlayed}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Побед</span>
                        <strong>{activeProgress.wins}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Winrate</span>
                        <strong>{activeProgress.winRate}%</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>XP за матчи</span>
                        <strong>{activeProgress.totalExpGained}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Убийств игроков</span>
                        <strong>{activeProgress.playerKills}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Убийств монстров</span>
                        <strong>{activeProgress.monsterKills}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Общий урон</span>
                        <strong>{activeProgress.damageTotal}</strong>
                    </div>
                </article>

                {!isExternalProfileMode && (
                    <article className={styles.card}>
                        <h3 className={styles.sectionTitle}>Ресурсы</h3>
                        <div className={styles.statRow}>
                            <span>Еда</span>
                            <strong>{profile.resources.food}</strong>
                        </div>
                        <div className={styles.statRow}>
                            <span>Вода</span>
                            <strong>{profile.resources.water}</strong>
                        </div>
                        <div className={styles.statRow}>
                            <span>Дерево</span>
                            <strong>{profile.resources.wood}</strong>
                        </div>
                        <div className={styles.statRow}>
                            <span>Камень</span>
                            <strong>{profile.resources.stone}</strong>
                        </div>
                        <div className={styles.statRow}>
                            <span>Железо</span>
                            <strong>{profile.resources.iron}</strong>
                        </div>
                    </article>
                )}

                {!isExternalProfileMode && (
                    <article className={styles.card}>
                        <h3 className={styles.sectionTitle}>База</h3>
                        <div className={styles.statRow}>
                            <span>Кузница</span>
                            <strong>
                                {profile.base.built
                                    ? `Построена (ур. ${profile.base.forgeLevel})`
                                    : "Не построена"}
                            </strong>
                        </div>
                        {!profile.base.built && (
                            <div className={styles.requirements}>
                                Нужно: дерево {profile.base.costs.wood}, камень{" "}
                                {profile.base.costs.stone}, железо{" "}
                                {profile.base.costs.iron}
                            </div>
                        )}
                        {profile.base.recipes.length > 0 && (
                            <div className={styles.recipeList}>
                                {profile.base.recipes.map((r) => (
                                    <div
                                        key={r.id}
                                        className={styles.recipeItem}
                                    >
                                        <strong>{r.name}</strong>
                                        <span>{r.description}</span>
                                    </div>
                                ))}
                            </div>
                        )}
                    </article>
                )}

                <article className={styles.card}>
                    <h3 className={styles.sectionTitle}>Боевые статы</h3>
                    <div className={styles.statRow}>
                        <span>Атака</span>
                        <strong>{activePlayer.attack}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Защита</span>
                        <strong>{activePlayer.defense}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Мобильность</span>
                        <strong>{activePlayer.mobility}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Ловкость</span>
                        <strong>{activePlayer.agility}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Обзор</span>
                        <strong>{activePlayer.sightRange}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Тип боя</span>
                        <strong>
                            {activePlayer.isRanged ? "Дальний" : "Ближний"}
                        </strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Дистанция атаки</span>
                        <strong>{activePlayer.attackRange}</strong>
                    </div>
                </article>
            </section>
        </div>
    );
}
