
// src/components/GameController.tsx

"use client";

import React, { useEffect } from "react";
import { useGame } from "../contexts/GameContextt";
import MapWithCamera from "./MapWithCamera";
import Controls from "./Controls";

export default function GameController() {
  const { state, dispatch } = useGame();

  // Обработчик перемещения активного игрока
  const handleMove = (direction: "up" | "down" | "left" | "right") => {
    const activePlayer = state.players.find((p) => p.id === state.currentPlayerId);
    if (!activePlayer) return;

    // Рассчитываем новую позицию
    let newPos = { ...activePlayer.position };
    if (direction === "up") newPos.y -= 1;
    else if (direction === "down") newPos.y += 1;
    else if (direction === "left") newPos.x -= 1;
    else if (direction === "right") newPos.x += 1;

    // Отправляем действие перемещения
    dispatch({ type: "MOVE_PLAYER", payload: { playerId: activePlayer.id, newPosition: newPos } });
  };

  // Обработчик кнопки "Действие"
  const handleAction = () => {
    const activePlayer = state.players.find((p) => p.id === state.currentPlayerId);
    if (!activePlayer) return;

    // Находим клетку, на которой находится активный игрок
    const currentCell = state.grid.find(
      (cell) =>
        cell.x === activePlayer.position.x &&
        cell.y === activePlayer.position.y
    );
    if (!currentCell) {
      console.log("Клетка не найдена");
      return;
    }

    // Если на клетке есть другой игрок (PvP)
    const otherPlayer = state.players.find(
      (p) =>
        p.id !== activePlayer.id &&
        p.position.x === activePlayer.position.x &&
        p.position.y === activePlayer.position.y
    );
    if (otherPlayer) {
      console.log("Начинаем бой: активный игрок атакует игрока", otherPlayer);
      // Здесь можно задиспатчить экшен начала боя:
      // dispatch({ type: "START_BATTLE", payload: { attacker: activePlayer, defender: otherPlayer } });
      return;
    }

    // Если на клетке есть монстр
    if (currentCell.monster) {
      console.log("Начинаем бой с монстром", currentCell.monster);
      // dispatch({ type: "START_BATTLE", payload: { attacker: activePlayer, defender: currentCell.monster } });
      return;
    }

    // Если на клетке есть ресурс – собираем его
    if (currentCell.resource) {
      console.log("Собираем ресурс", currentCell.resource);
      // dispatch({ type: "COLLECT_RESOURCE", payload: { playerId: activePlayer.id, resourceType: currentCell.resource.type, cellId: currentCell.id } });
      return;
    }

    // Если на клетке есть портал – пробуем выйти
    if (currentCell.isPortal) {
      console.log("Пытаемся выйти через портал");
      // dispatch({ type: "TRY_EXIT_PORTAL", payload: { playerId: activePlayer.id } });
      return;
    }

    console.log("На данной клетке нет интерактивных объектов");
  };

  // Обработка нажатий клавиш (стрелки и пробел)
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Отменяем действие по умолчанию для стрелок и пробела
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
      {/* Карта с камерой, которая всегда центрирована на активном игроке */}
      <MapWithCamera tileSize={80} viewportWidth={800} viewportHeight={600} />
      {/* Панель управления с кнопками перемещения и действия */}
      <Controls onMove={handleMove} onAction={handleAction} />
    </div>
  );
}
