"use client";

import React from "react";
import Tile from "./Tile";
import "../styles/map.css";

type Cell = {
  id: number;
  x: number;
  y: number;
  terrain: string;
  resource: { type: string; image: string; description: string } | null;
};

type MapProps = {
  grid: Cell[];
  playerPositions: { x: number; y: number }[];
  visionRange: number;
  mapWidth: number;
  mapHeight: number;
};

export default function Map({ grid, playerPositions, visionRange, mapWidth, mapHeight }: MapProps) {
  const activePlayerPos = playerPositions[0]; // пока что считаем первый игрок активный
  const { x, y } = activePlayerPos;

  const startX = Math.max(x - visionRange, 0);
  const endX = Math.min(x + visionRange, mapWidth - 1);
  const startY = Math.max(y - visionRange, 0);
  const endY = Math.min(y + visionRange, mapHeight - 1);

  const visibleTiles = grid.filter((cell) =>
    cell.x >= startX && cell.x <= endX && cell.y >= startY && cell.y <= endY
  );

  const rowsCount = endY - startY + 1;
  const colsCount = endX - startX + 1;
  const tileSize = 80;

  // проверяем, есть ли на тайле игрок
  const isPlayerOnCell = (cx: number, cy: number) =>
    playerPositions.some((p) => p.x === cx && p.y === cy);

  return (
    <div
      className="map"
      style={{
        display: "grid",
        gridTemplateColumns: `repeat(${colsCount}, ${tileSize}px)`,
        gridTemplateRows: `repeat(${rowsCount}, ${tileSize}px)`,
        marginLeft: "100px",
      }}
    >
      {visibleTiles.map((cell) => (
        <Tile
          key={cell.id}
          cell={cell}
          isPlayer={isPlayerOnCell(cell.x, cell.y)}
        />
      ))}
    </div>
  );
}
