
//==================================
// src/components/MapWithCamera.tsx
//==================================

"use client";

import React from "react";
import { useGame, PlayerState } from "../contexts/GameContextt";
import Map from "./Map";

interface MapWithCameraProps {
  tileSize: number;
  viewportWidth: number;
  viewportHeight: number;
  myPlayer: PlayerState;
}

export default function MapWithCamera({
  tileSize: inputTileSize,
  viewportWidth,
  viewportHeight,
  myPlayer,
}: MapWithCameraProps) {
  const { state } = useGame();
  const { grid, mapWidth, mapHeight, players, currentPlayerId } = state;

  // Используем позицию своего игрока для центрирования камеры
  const playerPosition = myPlayer?.position || { x: 0, y: 0 };
  // Можно брать vision именно из myPlayer
  const visionRange = myPlayer?.vision ?? 3;

  // Задаём размер тайла
  const tileSize = Number(inputTileSize) || 60;
  // Если размеры карты не переданы, используем значения по умолчанию
  const safeMapWidth = Number(mapWidth) || 15;
  const safeMapHeight = Number(mapHeight) || 15;

  // Размер gap между клетками
  const gap = 1;

  // Поправка для изображения игрока
  const playerImageOffsetX = 2;
  const playerImageOffsetY = 2;

  // Вычисляем смещение камеры так, чтобы МЫ (myPlayer) оказались в центре вьюпорта.
  let offsetX = viewportWidth / 2 - (playerPosition.x * (tileSize + gap) + tileSize / 2);
  let offsetY = viewportHeight / 2 - (playerPosition.y * (tileSize + gap) + tileSize / 2);

  // Ограничиваем смещение, чтобы камера не выходила за границы карты.
  const totalWidth = safeMapWidth * tileSize + (safeMapWidth - 1) * gap;
  const totalHeight = safeMapHeight * tileSize + (safeMapHeight - 1) * gap;
  offsetX = Math.min(0, Math.max(viewportWidth - totalWidth, offsetX));
  offsetY = Math.min(0, Math.max(viewportHeight - totalHeight, offsetY));

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
          mapWidth={safeMapWidth}
          mapHeight={safeMapHeight}
          tileSize={tileSize}
          gap={gap}
          visionRange={visionRange}
          playerPosition={playerPosition}
        />

        {/* Рендер изображений игроков */}
        {players.map((player) => (
          <img
            key={player.id}
            src={player.image}
            alt={player.name}
            title={player.name}
            style={{
              position: "absolute",
              left: player.position.x * (tileSize + gap) + playerImageOffsetX,
              top: player.position.y * (tileSize + gap) + playerImageOffsetY,
              width: tileSize,
              height: tileSize,
              border: player.id === currentPlayerId ? "2px solid gold" : "none",
              boxSizing: "border-box",
              zIndex: 10,
            }}
          />
        ))}
      </div>
    </div>
  );
}
