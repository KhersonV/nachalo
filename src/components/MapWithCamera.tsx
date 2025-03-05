// src/components/MapWithCamera.tsx
"use client";

import React from "react";
import { useGame } from "../contexts/GameContextt";
import Map from "./Map";

interface MapWithCameraProps {
  tileSize: number;
  viewportWidth: number;
  viewportHeight: number;
}

export default function MapWithCamera({
  tileSize,
  viewportWidth,
  viewportHeight,
}: MapWithCameraProps) {
  const { state } = useGame();
  const { grid, mapWidth, mapHeight, players, currentPlayerId } = state;
  const activePlayer = players.find((p) => p.id === currentPlayerId);

  // Размер gap между клетками (если используется в Map)
  const gap = 1;

  // Поправка для изображения игрока
  const playerImageOffsetX = 2; // сдвиг влево на 1 пиксель
  const playerImageOffsetY = 18; // сдвиг вверх на 10 пикселей

  // Вычисляем смещение камеры так, чтобы активный игрок оказался в центре вьюпорта.
  let offsetX = 0;
  let offsetY = 0;
  if (activePlayer) {
    offsetX =
      viewportWidth / 2 -
      (activePlayer.position.x * (tileSize + gap) + tileSize / 2);
    offsetY =
      viewportHeight / 2 -
      (activePlayer.position.y * (tileSize + gap) + tileSize / 2);

    // Ограничиваем смещение, чтобы камера не выходила за границы карты.
    const totalWidth = mapWidth * tileSize + (mapWidth - 1) * gap;
    const totalHeight = mapHeight * tileSize + (mapHeight - 1) * gap;
    offsetX = Math.min(0, Math.max(viewportWidth - totalWidth, offsetX));
    offsetY = Math.min(0, Math.max(viewportHeight - totalHeight, offsetY));
  } else {
    console.warn("Active player не найден, отображаем карту без смещения");
  }

  return (
    <div
      style={{
        width: viewportWidth,
        height: viewportHeight,
        overflow: "hidden",
        position: "relative",
        border: "2px solid #000",
      }}
    >
      <div
        style={{
          position: "absolute",
          top: offsetY,
          left: offsetX,
          transition: "top 0.3s, left 0.3s",
        }}
      >
        {/* Рендер карты */}
        <Map
          grid={grid}
          mapWidth={mapWidth}
          mapHeight={mapHeight}
          tileSize={tileSize}
          gap={gap}
        />

        {/* Рендер всех игроков на карте */}
        {players.map((player) => (
          <img
            key={player.id}
            src={player.image}
            alt={player.name}
            title={player.name}
            style={{
              position: "absolute",
              left:
                player.position.x * (tileSize + gap) + playerImageOffsetX,
              top:
                player.position.y * (tileSize + gap) + playerImageOffsetY,
              width: tileSize,
              height: tileSize,
              border:
                player.id === currentPlayerId ? "2px solid gold" : "none",
              boxSizing: "border-box",
              zIndex: 10,
            }}
          />
        ))}
      </div>
    </div>
  );
}
