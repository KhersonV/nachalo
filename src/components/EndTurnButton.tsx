//==================================
// src/components/EndTurnButton.tsx
//==================================

import React from "react";
import { debugLog } from "../utils/log";
import styles from "../styles/GameController.module.css";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8001";

interface EndTurnButtonProps {
    playerId: number;
    instanceId: string;
    onTurnEnded: (data: {
        active_user: number;
        turnNumber: number;
        energy: number;
    }) => void;
}

function EndTurnButton({
    playerId,
    instanceId,
    onTurnEnded,
}: EndTurnButtonProps) {
    const handleEndTurn = async () => {
        if (!instanceId) {
            console.error("instance_id is missing");
            return;
        }
        const storedUser = localStorage.getItem("user");
        const token = storedUser ? JSON.parse(storedUser).token : "";
        if (!token) {
            console.error("Token not found in localStorage");
            return;
        }
        try {
            const response = await fetch(`${API_BASE}/game/endTurn`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${token}`,
                },
                body: JSON.stringify({
                    user_id: playerId,
                    instance_id: instanceId,
                }),
            });

            if (!response.ok) {
                const errorMsg = await response.text();
                console.error("End turn error:", errorMsg);
                return;
            }

            const data = await response.json();
            debugLog("Server response to end turn:", data);
            // Call onTurnEnded with the data to update front-end state
            onTurnEnded({
                active_user: data.active_user,
                turnNumber: data.turn_number,
                energy: data.energy,
            });
        } catch (error) {
            console.error("Error performing request:", error);
        }
    };

    return (
        <button className={styles.endTurnButton} onClick={handleEndTurn}>
            End Turn
        </button>
    );
}

export default EndTurnButton;
