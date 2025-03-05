// src/components/Map.tsx
"use client";

import React from "react";

export interface Cell {
  id: number;
  x: number;
  y: number;
  tileCode: number;
  resource: any | null;
  isPortal?: boolean;
  monster?: any;
}

export interface MapProps {
  grid: Cell[];
  mapWidth: number;
  mapHeight: number;
  tileSize: number;
  gap: number;
}

export default function Map({
  grid,
  mapWidth,
  mapHeight,
  tileSize,
  gap,
}: MapProps) {
  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: `repeat(${mapWidth}, ${tileSize}px)`,
        gridTemplateRows: `repeat(${mapHeight}, ${tileSize}px)`,
        gap: `${gap}px`,
        border: "2px solid #333",
        marginTop: "1rem",
      }}
    >
      {grid.map((cell) => (
        <div
          key={cell.id}
          style={{
            width: `${tileSize}px`,
            height: `${tileSize}px`,
            backgroundColor: getTileColor(cell),
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            fontSize: "10px",
            color: "#fff",
          }}
        >
          {cell.id}
        </div>
      ))}
    </div>
  );
}

// Функция для определения цвета тайла по его значению.
function getTileColor(cell: Cell): string {
  switch (cell.tileCode) {
    case 49: // '1'
      return "#8B4513";
    case 48: // '0'
      return "#CCCCCC";
    case 32: // пробел
      return "#333333";
    case 77: // 'M'
      return "#FF0000";
    case 82: // 'R'
      return "#00AA00";
    case 80: // 'P'
      return "#0000FF";
    default:
      return "#999999";
  }
}
