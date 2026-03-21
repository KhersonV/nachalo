//==================================
// src/components/Controls.tsx
//==================================

"use client";

import React from "react";
import styles from "../styles/Controls.module.css";

type ControlsProps = {
    onMove: (direction: "up" | "down" | "left" | "right") => void;
    onAction: (action: string) => void;
};

export default function Controls({ onMove, onAction }: ControlsProps) {
    return (
        <div className={styles.controlsContainer}>
            <div className={styles.buttonRow}>
                <button
                    className={styles.controlButton}
                    onClick={() => onMove("up")}
                >
                    ↑
                </button>
            </div>
            <div className={styles.buttonRow}>
                <button
                    className={styles.controlButton}
                    onClick={() => onMove("left")}
                >
                    ←
                </button>
                <button
                    className={styles.controlButton}
                    onClick={() => onMove("down")}
                >
                    ↓
                </button>
                <button
                    className={styles.controlButton}
                    onClick={() => onMove("right")}
                >
                    →
                </button>
            </div>
            <div className={styles.actionRow}>
                <button
                    className={styles.actionButton}
                    onClick={() => onAction("attack")}
                >
                    Space
                </button>
            </div>
        </div>
    );
}
