
//==================================
// src/components/GameController.tsx
//==================================

"use client";

import React, { useEffect } from "react";
import { useSearchParams } from "next/navigation";
import { useGame, PlayerState } from "../contexts/GameContextt";
import { useAuth } from "../contexts/AuthContext";
import MapWithCamera from "./MapWithCamera";
import Controls from "./Controls";
import EndTurnButton from "./EndTurnButton";
import TurnIndicator from "./TurnIndicator";
import styles from "../styles/GameController.module.css";

export default function GameController() {
  const { state, dispatch } = useGame();
  const searchParams = useSearchParams();
  const instanceId = searchParams.get("instance_id") || "";
  const { user } = useAuth();

  // Определяем, какой игрок принадлежит текущему клиенту.
  const myPlayerId = user?.id;
  const myPlayer: PlayerState | undefined = state.players.find(
    (p) => p.id === myPlayerId
  );

  // Сравниваем, чей ход сейчас активен
  const isMyTurn = myPlayerId === state.currentPlayerId;

  // Функция для перемещения игрока на сервере
  async function movePlayer(newPosX: number, newPosY: number) {
    try {
      const storedUser = localStorage.getItem("user");
      const token = storedUser ? JSON.parse(storedUser).token : "";
      if (!token) {
        console.error("Токен не найден в localStorage");
        return null;
      }
      console.log("Полученный токен:", token);
      // Отправляем запрос по ID именно своего игрока, а не currentPlayerId
      const response = await fetch(
        `http://localhost:8001/game/player/${myPlayerId}/move?instance_id=${instanceId}`,
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

  const allowedTileCodes = [48, 80, 82, 112];
  const handleMove = async (direction: "up" | "down" | "left" | "right") => {
    if (!myPlayer) {
      console.warn("Мой игрок не найден");
      return;
    }
    // Только активный игрок может отправлять запросы на перемещение
    if (!isMyTurn) {
      console.warn("Сейчас не ваш ход");
      return;
    }
    let newPos = { ...myPlayer.position };

    if (direction === "up") {
      newPos.y -= 1;
    } else if (direction === "down") {
      newPos.y += 1;
    } else if (direction === "left") {
      newPos.x -= 1;
    } else if (direction === "right") {
      newPos.x += 1;
    }
    // Проверка клетки: наличие и проходимость
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
    // Проверяем, что клетка не занята другим игроком
    const otherPlayer = state.players.find(
      (p) =>
        p.id !== myPlayer.id &&
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
      dispatch({
        type: "MOVE_PLAYER",
        payload: { playerId: myPlayer.id, newPosition: newPos },
      });
    } else {
      console.error("Перемещение не выполнено из-за ошибки сервера.");
    }
  };
  
  const handleAction = () => {
    // Если ход не ваш, действия не выполняются
    if (!isMyTurn) {
      console.warn("Сейчас не ваш ход");
      return;
    }
    // Логика для атаки, сбора ресурсов и прочего
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
  const handleTurnEnded = (data: { activePlayer: number; turnNumber: number; energy: number }) => {
    dispatch({ 
      type: "SET_ACTIVE_PLAYER",
      payload: { activePlayer: data.activePlayer, turnNumber: data.turnNumber},
    });
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
  }, [state, myPlayer, isMyTurn]);

  return (
    <div className={styles.container}>
      <div className={styles.mapContainer}>
        {myPlayer ? (
          <MapWithCamera
            tileSize={80}
            viewportWidth={800}
            viewportHeight={600}
            myPlayer={myPlayer}
          />
        ) : (
          <p>Загрузка карты...</p>
        )}
      </div>
      <div className={styles.controlsContainer}>
        {isMyTurn ? (
          <>
            <Controls onMove={handleMove} onAction={handleAction} />
            <EndTurnButton
              playerId={myPlayerId!}
              instanceId={instanceId}
              onTurnEnded={handleTurnEnded}
            />
             <TurnIndicator /> 
          </>
        ) : (
          <div className={styles.waitingOverlay}>
            <p>Ожидание хода...</p>
          </div>
        )}
      </div>
    </div>
  );
}
