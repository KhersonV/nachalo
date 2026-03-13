"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import styles from "@/styles/GameStatsPage.module.css";

type Reward = {
    type?: string;
    amount?: number;
};

type PlayerStatsPayload = {
    instanceId: string;
    winnerType: "user" | "group";
    winnerId: number;
    player: {
        userId: number;
        name: string;
        expGained: number;
        playerKills: number;
        monsterKills: number;
        damageTotal: number;
        damageToPlayers: number;
        damageToMonsters: number;
        rewards?: Reward[] | string | null;
    };
};

function parseRewards(raw: PlayerStatsPayload["player"]["rewards"]): Reward[] {
    if (!raw) return [];
    if (Array.isArray(raw)) return raw;
    if (typeof raw === "string") {
        try {
            const parsed = JSON.parse(raw);
            return Array.isArray(parsed) ? parsed : [];
        } catch {
            return [];
        }
    }
    return [];
}

export default function GameStatsPage() {
    const router = useRouter();
    const [stats, setStats] = useState<PlayerStatsPayload | null>(null);

    useEffect(() => {
        if (typeof window === "undefined") return;
        const tryLoadStats = () => {
            const raw = sessionStorage.getItem("lastMatchPlayerStats");
            if (!raw) return false;
            try {
                const parsed = JSON.parse(raw) as PlayerStatsPayload;
                if (!parsed?.player) return false;
                setStats(parsed);
                return true;
            } catch {
                return false;
            }
        };

        if (tryLoadStats()) return;

        const onStatsReady = () => {
            tryLoadStats();
        };

        window.addEventListener("match-stats-ready", onStatsReady);

        const redirectTimer = window.setTimeout(() => {
            if (!tryLoadStats()) {
                router.replace("/mode");
            }
        }, 1500);

        return () => {
            window.removeEventListener("match-stats-ready", onStatsReady);
            window.clearTimeout(redirectTimer);
        };
    }, [router]);

    const rewards = useMemo(() => parseRewards(stats?.player.rewards), [stats]);

    if (!stats) {
        return <div className={styles.loading}>Загрузка статистики...</div>;
    }

    const winnerText =
        stats.winnerType === "group"
            ? `Победившая команда: ${stats.winnerId}`
            : `Победитель: игрок ${stats.winnerId}`;

    return (
        <main className={styles.page}>
            <section className={styles.card}>
                <h1 className={styles.title}>Статистика за матч</h1>
                <p className={styles.subtitle}>{winnerText}</p>

                <div className={styles.playerBlock}>
                    <h2 className={styles.playerName}>{stats.player.name}</h2>
                    <p className={styles.instance}>Матч: {stats.instanceId}</p>
                </div>

                <div className={styles.grid}>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Опыт</span>
                        <span className={styles.value}>
                            {stats.player.expGained}
                        </span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Убийства игроков</span>
                        <span className={styles.value}>
                            {stats.player.playerKills}
                        </span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Убийства монстров</span>
                        <span className={styles.value}>
                            {stats.player.monsterKills}
                        </span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Общий урон</span>
                        <span className={styles.value}>
                            {stats.player.damageTotal}
                        </span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Урон по игрокам</span>
                        <span className={styles.value}>
                            {stats.player.damageToPlayers}
                        </span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.label}>Урон по монстрам</span>
                        <span className={styles.value}>
                            {stats.player.damageToMonsters}
                        </span>
                    </div>
                </div>

                <section className={styles.rewards}>
                    <h3 className={styles.rewardsTitle}>Награды</h3>
                    {rewards.length === 0 ? (
                        <p className={styles.rewardsEmpty}>
                            Награды не получены
                        </p>
                    ) : (
                        <ul className={styles.rewardsList}>
                            {rewards.map((reward, idx) => (
                                <li
                                    key={`${reward.type ?? "reward"}-${idx}`}
                                    className={styles.rewardItem}
                                >
                                    {reward.type ?? "unknown"}:{" "}
                                    {reward.amount ?? 0}
                                </li>
                            ))}
                        </ul>
                    )}
                </section>

                <div className={styles.actions}>
                    <button
                        className={styles.button}
                        onClick={() => router.replace("/mode")}
                    >
                        В меню режимов
                    </button>
                </div>
            </section>
        </main>
    );
}
