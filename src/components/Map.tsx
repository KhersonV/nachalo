// Map.tsx
"use client";

import React from "react";
import Tile from "./Tile";
import { Cell } from "../logic/types"; // импортируем Cell из types.ts
import "../styles/map.css";

type MapProps = {
  grid: Cell[]; 
  playerPositions: { x: number; y: number }[]; 
  visionRange: number; 
  mapWidth: number; 
  mapHeight: number; 
  activePlayerIndex: number; 
};

export default function Map({
  grid,
  playerPositions,
  visionRange,
  mapWidth,
  mapHeight,
  activePlayerIndex,
}: MapProps) {
  const activePlayerPos = playerPositions[activePlayerIndex];
  const { x, y } = activePlayerPos;

  const startX = Math.max(x - visionRange, 0);
  const endX = Math.min(x + visionRange, mapWidth - 1);
  const startY = Math.max(y - visionRange, 0);
  const endY = Math.min(y + visionRange, mapHeight - 1);

  const visibleTiles = grid.filter(
    (cell) => cell.x >= startX && cell.x <= endX && cell.y >= startY && cell.y <= endY
  );

  const rowsCount = endY - startY + 1;
  const colsCount = endX - startX + 1;

  const tileSize = 80; 

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
      {visibleTiles.map((cell) => {
        const playersOnThisCell = playerPositions
          .map((pos, index) => (pos.x === cell.x && pos.y === cell.y ? index : -1))
          .filter((idx) => idx !== -1);

        return (
          <Tile
            key={cell.id}
            cell={cell}
            playersOnTile={playersOnThisCell}
          />
        );
      })}
    </div>
  );
}
