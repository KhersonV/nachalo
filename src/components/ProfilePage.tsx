"use client";

import React, { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "../contexts/AuthContext";
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

const API_GAME = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8001";

export default function ProfilePage() {
    const router = useRouter();
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
            setEditImage(data.player.image || "");
        } catch (e: any) {
            setError(e?.message || "Ошибка загрузки профиля");
        } finally {
            setLoading(false);
        }
    }, [user, fetchProfile]);

    useEffect(() => {
        loadProfile();
    }, [loadProfile]);

    useEffect(() => {
        if (!user?.token) return;

        let alive = true;

        const refreshSilently = async () => {
            try {
                const data = await fetchProfile();
                if (!alive || !data) return;
                setProfile(data);
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
    }, [user, fetchProfile]);

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

    const expPercent = profile.player.maxExperience
        ? Math.min(
              100,
              Math.round(
                  (profile.player.experience / profile.player.maxExperience) *
                      100,
              ),
          )
        : 0;

    const activityLabel =
        profile.player.activityStatus === "in_match"
            ? "В игре"
            : profile.player.activityStatus === "in_lobby"
              ? "В лобби"
              : "Офлайн";

    const activityClass =
        profile.player.activityStatus === "in_match"
            ? styles.statusInMatch
            : profile.player.activityStatus === "in_lobby"
              ? styles.statusInLobby
              : styles.statusOffline;

    return (
        <div className={styles.page}>
            <div className={styles.topRow}>
                <h2 className={styles.title}>Профиль игрока</h2>
                <button
                    className={styles.backBtn}
                    onClick={() => router.push("/mode")}
                >
                    К режимам
                </button>
            </div>

            <section className={styles.card}>
                <div className={styles.identityRow}>
                    <img
                        className={styles.avatar}
                        src={profile.player.image || "/player-1.webp"}
                        alt="avatar"
                    />
                    <div className={styles.identityMeta}>
                        <div className={styles.name}>{profile.player.name}</div>
                        <div
                            className={`${styles.statusBadge} ${activityClass}`}
                        >
                            {activityLabel}
                        </div>
                        <div className={styles.subline}>
                            Класс: {profile.player.characterType}
                        </div>
                        <div className={styles.subline}>
                            Баланс: {profile.player.balance}
                        </div>
                    </div>
                </div>

                <div className={styles.expWrap}>
                    <div className={styles.expLabel}>
                        Уровень {profile.player.level} •{" "}
                        {profile.player.experience}/
                        {profile.player.maxExperience} XP
                    </div>
                    <div className={styles.expBarTrack}>
                        <div
                            className={styles.expBarFill}
                            style={{ width: `${expPercent}%` }}
                        />
                    </div>
                </div>

                <div className={styles.editGrid}>
                    <label className={styles.label}>
                        Ник
                        <input
                            className={styles.input}
                            value={editName}
                            onChange={(e) => setEditName(e.target.value)}
                            maxLength={32}
                        />
                    </label>
                    <label className={styles.label}>
                        URL аватара
                        <input
                            className={styles.input}
                            value={editImage}
                            onChange={(e) => setEditImage(e.target.value)}
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
                    {info && <span className={styles.info}>{info}</span>}
                    {error && <span className={styles.error}>{error}</span>}
                </div>
            </section>

            <section className={styles.grid}>
                <article className={styles.card}>
                    <h3 className={styles.sectionTitle}>Прогресс</h3>
                    <div className={styles.statRow}>
                        <span>Матчей</span>
                        <strong>{profile.progress.matchesPlayed}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Побед</span>
                        <strong>{profile.progress.wins}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Winrate</span>
                        <strong>{profile.progress.winRate}%</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>XP за матчи</span>
                        <strong>{profile.progress.totalExpGained}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Убийств игроков</span>
                        <strong>{profile.progress.playerKills}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Убийств монстров</span>
                        <strong>{profile.progress.monsterKills}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Общий урон</span>
                        <strong>{profile.progress.damageTotal}</strong>
                    </div>
                </article>

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
                                <div key={r.id} className={styles.recipeItem}>
                                    <strong>{r.name}</strong>
                                    <span>{r.description}</span>
                                </div>
                            ))}
                        </div>
                    )}
                </article>

                <article className={styles.card}>
                    <h3 className={styles.sectionTitle}>Боевые статы</h3>
                    <div className={styles.statRow}>
                        <span>Атака</span>
                        <strong>{profile.player.attack}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Защита</span>
                        <strong>{profile.player.defense}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Мобильность</span>
                        <strong>{profile.player.mobility}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Ловкость</span>
                        <strong>{profile.player.agility}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Обзор</span>
                        <strong>{profile.player.sightRange}</strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Тип боя</span>
                        <strong>
                            {profile.player.isRanged ? "Дальний" : "Ближний"}
                        </strong>
                    </div>
                    <div className={styles.statRow}>
                        <span>Дистанция атаки</span>
                        <strong>{profile.player.attackRange}</strong>
                    </div>
                </article>
            </section>
        </div>
    );
}
