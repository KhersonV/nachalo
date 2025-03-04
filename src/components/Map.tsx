// src/components/Map.tsx
"use client";

import React from "react";

export interface Cell {
  id: number;
  x: number;
  y: number;
  tileCode: number;
  resource: any | null; // уточните тип, если нужно
  isPortal?: boolean;
  monster?: any;
}

export interface MapProps {
  grid: Cell[]; // массив объектов Cell
  playerPositions: { x: number; y: number }[];
  visionRange: number;
  mapWidth: number;
  mapHeight: number;
  activePlayerIndex: number;
}

export default function Map({
  grid,
  playerPositions,
  visionRange,
  mapWidth,
  mapHeight,
  activePlayerIndex,
}: MapProps) {
  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: `repeat(${mapWidth}, 60px)`,
        gridTemplateRows: `repeat(${mapHeight}, 60px)`,
        gap: "1px",
        border: "2px solid #333",
        marginTop: "1rem",
      }}
    >
      {grid.map((cell) => (
        <div
          key={cell.id}
          style={{
            width: "60px",
            height: "60px",
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
      return "#8B4513"; // коричневый (граница)
    case 48: // '0'
      return "#CCCCCC"; // светло-серый (проходимый)
    case 32: // пробел
      return "#333333"; // темно-серый (непроходимый)
    case 77: // 'M'
      return "#FF0000"; // красный (монстр)
    case 82: // 'R'
      return "#00AA00"; // зелёный (ресурс)
    case 80: // 'P'
      return "#0000FF"; // синий (портал/старт)
    default:
      return "#999999"; // по умолчанию
  }
}
