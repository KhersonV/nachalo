
//==================================
// src/components/GameController.tsx
//==================================

"use client";

import React, { useEffect } from "react";
import { useSearchParams } from "next/navigation";
import { useGame } from "../contexts/GameContextt";
import MapWithCamera from "./MapWithCamera";
import Controls from "./Controls";
import EndTurnButton from "./EndTurnButton";
import styles from "../styles/GameController.module.css";

export default function GameController() {
  const { state, dispatch } = useGame();
  const searchParams = useSearchParams();
  const instanceId = searchParams.get("instance_id") || "";

  // Функция для перемещения игрока на сервере
  async function movePlayer(newPosX: number, newPosY: number) {
    try {
      // Получаем токен из localStorage
      const storedUser = localStorage.getItem("user");
      const token = storedUser ? JSON.parse(storedUser).token : "";
      if (!token) {
        console.error("Токен не найден в localStorage");
        return null;
      }
      console.log("Полученный токен:", token);
      const response = await fetch(
        `http://localhost:8001/game/player/${state.currentPlayerId}/move?instance_id=${instanceId}`,
        {
          method: "POST",
          headers: { 
            "Content-Type": "application/json",
            "Authorization": `Bearer ${token}`
          },
          body: JSON.stringify({ new_pos_x: newPosX, new_pos_y: newPosY }),
        }
      );
      if (!response.ok) {
        const errorText = await response.text();
        console.error("Ошибка перемещения:", errorText);
        return null;
      }
      const updatedPlayer = await response.json();
      console.log("Позиция обновлена на сервере:", updatedPlayer);
      return updatedPlayer;
    } catch (error) {
      console.error("Ошибка запроса перемещения:", error);
      return null;
    }
  }

  const allowedTileCodes = [48, 80, 77, 82, 112];
  const handleMove = async (direction: "up" | "down" | "left" | "right") => {
    const activePlayer = state.players.find((p) => p.id === state.currentPlayerId);
    if (!activePlayer) {
      console.warn("Активный игрок не найден");
      return;
    }
  
    let newPos = { ...activePlayer.position };
  
    if (direction === "up") {
      newPos.y -= 1;
    } else if (direction === "down") {
      newPos.y += 1;
    } else if (direction === "left") {
      newPos.x -= 1;
    } else if (direction === "right") {
      newPos.x += 1;
    }
  
    // Проверка клетки, занятости, проходимости и т.п.
    const targetCell = state.grid.find(
      (cell) => cell.x === newPos.x && cell.y === newPos.y
    );
    if (!targetCell) {
      console.warn("Целевая клетка не найдена для координат", newPos);
      return;
    }
    if (!allowedTileCodes.includes(targetCell.tileCode)) {
      console.warn("Невозможно переместиться: клетка непроходимая", targetCell);
      return;
    }
    const otherPlayer = state.players.find(
      (p) =>
        p.id !== activePlayer.id &&
        p.position.x === newPos.x &&
        p.position.y === newPos.y
    );
    if (otherPlayer) {
      console.warn("Невозможно переместиться: клетка занята другим игроком", otherPlayer);
      return;
    }
  
    // Отправляем запрос и ждём подтверждения от сервера
    const updatedPlayer = await movePlayer(newPos.x, newPos.y);
    if (updatedPlayer) {
      // После подтверждения сервера обновляем локальное состояние
      dispatch({
        type: "MOVE_PLAYER",
        payload: { playerId: activePlayer.id, newPosition: newPos },
      });
    } else {
      console.error("Перемещение не выполнено из-за ошибки сервера.");
    }
  };
  
  const handleAction = () => {
    const activePlayer = state.players.find((p) => p.id === state.currentPlayerId);
    if (!activePlayer) return;
    const currentCell = state.grid.find(
      (cell) =>
        cell.x === activePlayer.position.x &&
        cell.y === activePlayer.position.y
    );
    if (!currentCell) {
      console.log("Клетка не найдена");
      return;
    }
    if (currentCell.monster) {
      console.log("Начинаем бой с монстром", currentCell.monster);
      return;
    }
    if (currentCell.resource) {
      console.log("Собираем ресурс", currentCell.resource);
      return;
    }
    if (currentCell.isPortal) {
      console.log("Пытаемся выйти через портал");
      return;
    }
    console.log("На данной клетке нет интерактивных объектов", currentCell);
  };

  // Функция обновления активного игрока после завершения хода
  const handleTurnEnded = (newActivePlayer: number) => {
    dispatch({ type: "SET_ACTIVE_PLAYER", payload: newActivePlayer });
  };

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", " "].includes(e.key)) {
        e.preventDefault();
      }
      if (e.key === "ArrowUp") {
        handleMove("up");
      } else if (e.key === "ArrowDown") {
        handleMove("down");
      } else if (e.key === "ArrowLeft") {
        handleMove("left");
      } else if (e.key === "ArrowRight") {
        handleMove("right");
      } else if (e.key === " ") {
        handleAction();
      }
    };
    
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [state]);

  return (
    <div className={styles.container}>
      <div className={styles.mapContainer}>
        <MapWithCamera tileSize={80} viewportWidth={800} viewportHeight={600} />
      </div>
      <div className={styles.controlsContainer}>
        <Controls onMove={handleMove} onAction={handleAction} />
        <EndTurnButton
          playerId={state.currentPlayerId}
          instanceId={instanceId}
          onTurnEnded={handleTurnEnded}
        />
      </div>
    </div>
  );
}
