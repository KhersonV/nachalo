// src/components/GameController.tsx
"use client";

import React, { useEffect } from "react";
import { useSearchParams } from "next/navigation";
import { useGame } from "../contexts/GameContextt";
import MapWithCamera from "./MapWithCamera";
import Controls from "./Controls";

export default function GameController() {
  const { state, dispatch } = useGame();
  const searchParams = useSearchParams();
  const instanceId = searchParams.get("instance_id") || "";

  // Функция, отправляющая запрос на сервер для перемещения игрока
  async function movePlayer(newPosX: number, newPosY: number) {
    try {
      const response = await fetch(
        `http://localhost:8001/game/player/${state.currentPlayerId}/move?instance_id=${instanceId}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
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

  // Список разрешённых tileCode для перемещения:
  // 48 - стандартная проходимость ('0'),
  // 80 - старт/портал ('P'),
  // 77 - монстр ('M'),
  // 82 - ресурс ('R'),
  // 112 - портал ('p')
  const allowedTileCodes = [48, 80, 77, 82, 112];

  // Обработчик перемещения активного игрока с проверкой коллизии
  const handleMove = async (direction: "up" | "down" | "left" | "right") => {
    const activePlayer = state.players.find((p) => p.id === state.currentPlayerId);
    if (!activePlayer) {
      console.warn("Активный игрок не найден");
      return;
    }

    // Начинаем с текущей позиции
    let newPos = { ...activePlayer.position };

    // Изменяем координаты в зависимости от направления
    if (direction === "up") {
      newPos.y -= 1;
      console.log("Попытка движения вверх, новые координаты:", newPos);
    } else if (direction === "down") {
      newPos.y += 1;
      console.log("Попытка движения вниз, новые координаты:", newPos);
    } else if (direction === "left") {
      newPos.x -= 1;
      console.log("Попытка движения влево, новые координаты:", newPos);
    } else if (direction === "right") {
      newPos.x += 1;
      console.log("Попытка движения вправо, новые координаты:", newPos);
    }

    // Проверяем, существует ли целевая клетка на карте
    const targetCell = state.grid.find(
      (cell) => cell.x === newPos.x && cell.y === newPos.y
    );
    if (!targetCell) {
      console.warn("Целевая клетка не найдена для координат", newPos);
      return;
    }

    // Проверяем, что tileCode целевой клетки входит в список разрешённых
    if (!allowedTileCodes.includes(targetCell.tileCode)) {
      console.warn("Невозможно переместиться: клетка непроходимая", targetCell);
      return;
    }

    // Проверяем, не занята ли клетка другим игроком
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

    // Оптимистично обновляем локальное состояние
    dispatch({
      type: "MOVE_PLAYER",
      payload: { playerId: activePlayer.id, newPosition: newPos },
    });

    // Отправляем запрос на сервер для обновления позиции
    await movePlayer(newPos.x, newPos.y);
  };

  // Обработчик кнопки "Действие"
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

  // Обработка нажатий клавиш
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
    <div style={{ position: "relative", width: "800px", height: "600px" }}>
      <MapWithCamera tileSize={80} viewportWidth={800} viewportHeight={600} />
      <Controls onMove={handleMove} onAction={handleAction} />
    </div>
  );
}
