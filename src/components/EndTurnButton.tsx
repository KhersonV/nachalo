//==================================
// src/components/EndTurnButton.tsx
//==================================

import React from "react";
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
            console.error("instance_id отсутствует");
            return;
        }
        const storedUser = localStorage.getItem("user");
        const token = storedUser ? JSON.parse(storedUser).token : "";
        if (!token) {
            console.error("Токен не найден в localStorage");
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
                console.error("Ошибка завершения хода:", errorMsg);
                return;
            }

            const data = await response.json();
            console.log("Ответ от сервера на завершение хода:", data);
            // Вызовем onTurnEnded с данными, чтобы обновить состояние на фронте
            onTurnEnded({
                active_user: data.active_user,
                turnNumber: data.turn_number,
                energy: data.energy,
            });
        } catch (error) {
            console.error("Ошибка при выполнении запроса:", error);
        }
    };

    return (
        <button className={styles.endTurnButton} onClick={handleEndTurn}>
            Конец хода
        </button>
    );
}

export default EndTurnButton;
