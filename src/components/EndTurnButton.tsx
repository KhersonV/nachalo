
//==================================
// src/components/EndTurnButton.tsx
//==================================

import React from 'react';

interface EndTurnButtonProps {
  playerId: number;
  instanceId: string;
  onTurnEnded: (newActivePlayer: number) => void;
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
    console.log("Отправка запроса завершения хода от игрока:", playerId, "с токеном:", token, "и instance_id:", instanceId);
    try {
      console.log("Токен из localStorage:", token);
      

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
      console.log("Новый активный игрок:", data.active_player);
      onTurnEnded(data.active_player);
    } catch (error) {
      console.error("Ошибка при выполнении запроса:", error);
    }
  };
  
  return <button onClick={handleEndTurn}>Конец хода</button>;
}
     
export default EndTurnButton;
