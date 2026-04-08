//=====================================
// src/components/TurnIndicator.tsx
//=====================================

"use client";

import React from "react";
import { useSelector } from "react-redux";
import type { RootState } from "../store";
import styles from "../styles/GameController.module.css";

const TurnIndicator = () => {
    const turnNumber = useSelector((state: RootState) => state.game.turnNumber);

    return (
        <div className={styles.turnIndicator}>
            <p>Turn: {turnNumber}</p>
        </div>
    );
};

export default TurnIndicator;
