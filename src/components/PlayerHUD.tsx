//==============================
// src/components/PlayerHUD.tsx
//==============================

"use client";

import React from "react";
import {
    getClassReflexEffectDescription,
    getClassReflexEffectName,
    getReflexProcChance,
} from "../utils/reflex";
import styles from "../styles/GameController.module.css";

interface PlayerHUDProps {
    health: number;
    maxHealth: number;
    energy: number;
    maxEnergy: number;
    agility?: number;
    characterType?: string;
    isRanged?: boolean;
    attackRange?: number;
    groupId?: number;
}

export default React.memo(function PlayerHUD({
    health,
    maxHealth,
    energy,
    maxEnergy,
    agility,
    characterType,
    isRanged,
    attackRange,
    groupId,
}: PlayerHUDProps) {
    const healthPercent = Math.round((health / maxHealth) * 100);
    const energyPercent = Math.round((energy / maxEnergy) * 100);
    const attackTypeLabel = isRanged ? "Ranged" : "Melee";
    const attackRangeLabel = isRanged ? (attackRange ?? 1) : 1;
    const reflexChance =
        typeof agility === "number" ? getReflexProcChance(agility) : null;
    const reflexEffectName = getClassReflexEffectName(characterType);
    const reflexDescription = getClassReflexEffectDescription(characterType);
    const reflexTitle = `Effect: ${reflexEffectName}. ${reflexDescription}`;

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
            <div className={`${styles.hudMetaRow} ${styles.hudCombatMeta}`}>
                <span className={styles.hudMetaLabel}>Attack Type</span>
                <span className={styles.hudMetaValue}>{attackTypeLabel}</span>
            </div>
            <div className={`${styles.hudMetaRow} ${styles.hudCombatMeta}`}>
                <span className={styles.hudMetaLabel}>Range</span>
                <span className={styles.hudMetaValue}>{attackRangeLabel}</span>
            </div>
            {typeof agility === "number" && (
                <div
                    className={`${styles.hudMetaRow} ${styles.hudCombatMeta}`}
                    title={reflexTitle}
                >
                    <span className={styles.hudMetaLabel}>Reflex</span>
                    <span className={styles.hudMetaValue}>
                        {agility} · {reflexChance}%
                    </span>
                </div>
            )}
            {typeof groupId === "number" && (
                <div className={styles.hudMetaRow}>
                    <span className={styles.hudMetaLabel}>Group</span>
                    <span className={styles.hudMetaValue}>{groupId}</span>
                </div>
            )}
        </div>
    );
});
