//==============================
// src/components/PlayerHUD.tsx
//==============================

"use client";

import React from "react";
import styles from "../styles/GameController.module.css";

interface PlayerHUDProps {
    health: number;
    maxHealth: number;
    energy: number;
    maxEnergy: number;
    isRanged?: boolean;
    attackRange?: number;
    groupId?: number;
}

export default React.memo(function PlayerHUD({
    health,
    maxHealth,
    energy,
    maxEnergy,
    isRanged,
    attackRange,
    groupId,
}: PlayerHUDProps) {
    const healthPercent = Math.round((health / maxHealth) * 100);
    const energyPercent = Math.round((energy / maxEnergy) * 100);
    const attackTypeLabel = isRanged ? "Дальний" : "Ближний";
    const attackRangeLabel = isRanged ? (attackRange ?? 1) : 1;

    return (
        <div className={styles.hud}>
            <div className={styles.hudRow}>
                <span>HP:</span>
                <div className={styles.progressBar}>
                    <div
                        className={`${styles.progressFill} ${styles.progressFillHealth}`}
                        style={{ width: `${healthPercent}%` }}
                    />
                </div>
                <span>
                    {health} / {maxHealth}
                </span>
            </div>
            <div className={styles.hudRow}>
                <span>Energy:</span>
                <div className={styles.progressBar}>
                    <div
                        className={`${styles.progressFill} ${styles.progressFillEnergy}`}
                        style={{ width: `${energyPercent}%` }}
                    />
                </div>
                <span>
                    {energy} / {maxEnergy}
                </span>
            </div>
            <div className={styles.hudMetaRow}>
                <span className={styles.hudMetaLabel}>Тип боя</span>
                <span className={styles.hudMetaValue}>{attackTypeLabel}</span>
            </div>
            <div className={styles.hudMetaRow}>
                <span className={styles.hudMetaLabel}>Дальность</span>
                <span className={styles.hudMetaValue}>{attackRangeLabel}</span>
            </div>
            {typeof groupId === "number" && (
                <div className={styles.hudMetaRow}>
                    <span className={styles.hudMetaLabel}>Группа</span>
                    <span className={styles.hudMetaValue}>{groupId}</span>
                </div>
            )}
        </div>
    );
});
