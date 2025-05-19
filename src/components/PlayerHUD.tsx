
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
  return (
    <div className={styles.hud}>
      <p>HP: {health} / {maxHealth}</p>
      <p>Energy: {energy} / {maxEnergy}</p>
    </div>
  );
});

