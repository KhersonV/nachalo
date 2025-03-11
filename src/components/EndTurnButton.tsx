
//==================================
// src/components/EndTurnButton.tsx
//==================================

import React from 'react';

interface EndTurnButtonProps {
  playerId: number;
  instanceId: string;
  onTurnEnded: (data: { activePlayer: number; turnNumber: number; energy: number }) => void;
}

function EndTurnButton({ playerId, instanceId, onTurnEnded }: EndTurnButtonProps) {
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
      const response = await fetch("http://localhost:8001/game/endTurn", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${token}`,
        },
        body: JSON.stringify({ player_id: playerId, instance_id: instanceId }),
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
        activePlayer: data.active_player,
        turnNumber: data.turn_number,
        energy: data.energy,
      });
    } catch (error) {
      console.error("Ошибка при выполнении запроса:", error);
    }
  };
  
  return <button onClick={handleEndTurn}>Конец хода</button>;
}

export default EndTurnButton;
