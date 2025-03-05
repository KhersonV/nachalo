// src/components/MapWithCamera.tsx
"use client";

import React from "react";
import { useGame } from "../contexts/GameContextt";
import Map from "./Map";

type MapWithCameraProps = {
  tileSize: number;
  viewportWidth: number;
  viewportHeight: number;
};

export default function MapWithCamera({
  tileSize,
  viewportWidth,
  viewportHeight,
}: MapWithCameraProps) {
  const { state } = useGame();
  const activePlayer = state.players.find((p) => p.id === state.currentPlayerId);

  if (!activePlayer) {
    console.warn("Active player не найден, отображаем карту без смещения");
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
        <Map
          grid={state.grid}
          playerPositions={state.players.map((p) => p.position)}
          visionRange={5}
          mapWidth={state.mapWidth}
          mapHeight={state.mapHeight}
          activePlayerIndex={0}
        />
      </div>
    );
  }

  // Вычисляем смещение камеры так, чтобы активный игрок оказался в центре вьюпорта.
  let offsetX =
    viewportWidth / 2 - activePlayer.position.x * tileSize - tileSize / 2;
  let offsetY =
    viewportHeight / 2 - activePlayer.position.y * tileSize - tileSize / 2;

  // Ограничиваем смещение, чтобы камера не выходила за границы карты.
  const totalWidth = state.mapWidth * tileSize;
  const totalHeight = state.mapHeight * tileSize;

  // offsetX: не должен быть больше 0 (слева) и меньше (viewportWidth - totalWidth)
  offsetX = Math.min(0, Math.max(viewportWidth - totalWidth, offsetX));
  // offsetY: аналогично
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
        <Map
          grid={state.grid}
          playerPositions={state.players.map((p) => p.position)}
          visionRange={activePlayer.visionRange}
          mapWidth={state.mapWidth}
          mapHeight={state.mapHeight}
          activePlayerIndex={state.players.findIndex((p) => p.id === state.currentPlayerId)}
        />
      </div>
    </div>
  );
}
