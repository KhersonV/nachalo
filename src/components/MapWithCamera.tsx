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

  // Определяем активного игрока по currentPlayerId
  const activePlayerIndex = state.players.findIndex(
    (p) => p.id === state.currentPlayerId
  );
  const activePlayer = state.players[activePlayerIndex];

  // Если активного игрока нет, рендерим карту без смещения
  if (!activePlayer) {
    return (
      <Map
        grid={state.grid}
        playerPositions={state.players.map((p) => p.position)}
        visionRange={5} // значение по умолчанию
        mapWidth={state.mapWidth}
        mapHeight={state.mapHeight}
        activePlayerIndex={0}
      />
    );
  }

  // Вычисляем смещение камеры так, чтобы активный игрок оказался в центре вьюпорта.
  const offsetX =
    viewportWidth / 2 - activePlayer.position.x * tileSize - tileSize / 2;
  const offsetY =
    viewportHeight / 2 - activePlayer.position.y * tileSize - tileSize / 2;

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
          activePlayerIndex={activePlayerIndex}
        />
      </div>
    </div>
  );
}
