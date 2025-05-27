
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
}

export default React.memo(function PlayerHUD({
  health,
  maxHealth,
  energy,
  maxEnergy,
}: PlayerHUDProps) {

  const healthPercent = Math.round((health / maxHealth) * 100);
  const energyPercent = Math.round((energy / maxEnergy) * 100);


  return (
    <div className={styles.hud}>
      <div className={styles.hudRow}>
        <span>HP:</span>
        <div className={styles.progressBar}>
          <div
            className={styles.progressFill}
            style={{ width: `${healthPercent}%` }}
          />
        </div>
        <span>{health} / {maxHealth}</span>
      </div>
      <div className={styles.hudRow}>
        <span>Energy:</span>
        <div className={styles.progressBar}>
          <div
            className={styles.progressFill}
            style={{ width: `${energyPercent}%` }}
          />
        </div>
        <span>{energy} / {maxEnergy}</span>
      </div>
    </div>
  );
});


