//==============================
// src/components/ObjectHUD.tsx
//==============================

"use client";

import React from "react";
import styles from "../styles/ObjectHUD.module.css";

export interface ObjectHUDProps {
    type: "monster" | "structure" | "player" | "object";
    name: string;
    details?: string;
    groupId?: number;
    health?: number;
    maxHealth?: number;
    energy?: number;
    maxEnergy?: number;
    attack?: number;
    defense?: number;
    sightRange?: number;
    structureType?: "scout_tower" | "turret" | "wall";
    onProfileClick?: () => void;
    onClose?: () => void;
}

const TYPE_LABELS: Record<ObjectHUDProps["type"], string> = {
    player: "Игрок",
    monster: "Монстр",
    object: "Объект",
    structure: "Постройка",
};

function HealthBar({
    health,
    maxHealth,
}: {
    health: number;
    maxHealth: number;
}) {
    const pct = Math.max(
        0,
        Math.min(100, Math.round((health / maxHealth) * 100)),
    );
    const fillClass =
        pct <= 25
            ? `${styles.progressFill} ${styles.progressFillHealthLow}`
            : pct <= 60
              ? `${styles.progressFill} ${styles.progressFillHealthMid}`
              : `${styles.progressFill} ${styles.progressFillHealth}`;
    return (
        <div className={styles.hudRow}>
            <span>HP:</span>
            <div className={styles.progressBar}>
                <div className={fillClass} style={{ width: `${pct}%` }} />
            </div>
            <span>
                {health} / {maxHealth}
            </span>
        </div>
    );
}

export const ObjectHUD: React.FC<ObjectHUDProps> = ({
    type,
    name,
    details,
    health,
    maxHealth,
    energy,
    maxEnergy,
    attack,
    defense,
    sightRange,
    structureType,
    groupId,
    onProfileClick,
    onClose,
}) => {
    const hasStats =
        attack !== undefined ||
        defense !== undefined ||
        (structureType === "scout_tower" && sightRange !== undefined);

    return (
        <>
            <div className={styles.header}>
                <span className={styles.typeBadge}>{TYPE_LABELS[type]}</span>
                <span className={styles.name}>{name}</span>
                {onClose && (
                    <button
                        className={styles.closeBtn}
                        onClick={onClose}
                        aria-label="Закрыть"
                    >
                        ×
                    </button>
                )}
            </div>

            {details && <p className={styles.details}>{details}</p>}

            {health !== undefined && maxHealth !== undefined && (
                <HealthBar health={health} maxHealth={maxHealth} />
            )}
            {health !== undefined && maxHealth === undefined && (
                <div className={styles.statRow}>
                    <span className={styles.statLabel}>HP</span>
                    <div />
                    <span className={styles.statValue}>{health}</span>
                </div>
            )}

            {energy !== undefined && maxEnergy !== undefined && (
                <div className={styles.statRow}>
                    <span className={styles.statLabel}>Энергия</span>
                    <div />
                    <span className={styles.statValue}>
                        {energy} / {maxEnergy}
                    </span>
                </div>
            )}

            {hasStats && <hr className={styles.divider} />}

            {typeof groupId === "number" && (
                <div className={styles.statRow}>
                    <span className={styles.statLabel}>Группа</span>
                    <div />
                    <span className={styles.statValue}>{groupId}</span>
                </div>
            )}

            {hasStats && (
                <div className={styles.smallStatGrid}>
                    {attack !== undefined && (
                        <div className={styles.smallStat}>
                            <span className={styles.smallStatLabel}>Атака</span>
                            <span className={styles.smallStatValue}>
                                {attack}
                            </span>
                        </div>
                    )}
                    {defense !== undefined && (
                        <div className={styles.smallStat}>
                            <span className={styles.smallStatLabel}>
                                Защита
                            </span>
                            <span className={styles.smallStatValue}>
                                {defense}
                            </span>
                        </div>
                    )}
                    {structureType === "scout_tower" &&
                        sightRange !== undefined && (
                            <div className={styles.smallStat}>
                                <span className={styles.smallStatLabel}>
                                    Обзор
                                </span>
                                <span className={styles.smallStatValue}>
                                    {sightRange}
                                </span>
                            </div>
                        )}
                </div>
            )}

            {onProfileClick && (
                <button className={styles.profileBtn} onClick={onProfileClick}>
                    К профилю
                </button>
            )}
        </>
    );
};
