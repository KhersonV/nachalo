
//==================================
// src/components/Map.tsx
//==================================

"use client";

import React from "react";

export interface Cell {
  id: number;
  x: number;
  y: number;
  tileCode: number;
  resource: { image?: string } | null;
  isPortal?: boolean;
  monster?: { image?: string } | null;
}

export interface MapProps {
  grid: Cell[];
  mapWidth: number;
  mapHeight: number;
  tileSize: number;
  gap: number;
  visionRange: number;
  playerPosition: { x: number; y: number };
}

export default function Map({
  grid,
  mapWidth,
  mapHeight,
  tileSize,
  gap,
  visionRange,
  playerPosition,
}: MapProps) {
  
  const isCellVisible = (cell: Cell): boolean => {
    const dx = Math.abs(cell.x - playerPosition.x);
    const dy = Math.abs(cell.y - playerPosition.y);
    return dx <= visionRange && dy <= visionRange;
  };

  const fullWidth = mapWidth * tileSize + (mapWidth - 1) * gap;
  const fullHeight = mapHeight * tileSize + (mapHeight - 1) * gap;

  return (
    <div
      style={{
        position: "relative",
        width: `${fullWidth}px`,
        height: `${fullHeight}px`,
        display: "grid",
        gridTemplateColumns: `repeat(${mapWidth}, ${tileSize}px)`,
        gridTemplateRows: `repeat(${mapHeight}, ${tileSize}px)`,
        gap: `${gap}px`,
        border: "2px solid #333",
      }}
    >
      {grid.map((cell) => (
        <div
          key={cell.id}
          style={{
            width: `${tileSize}px`,
            height: `${tileSize}px`,
            backgroundColor: getTileColor(cell),
            opacity: isCellVisible(cell) ? 1 : 0, // затемняем клетки вне видимости
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            fontSize: "10px",
            color: "#fff",
            position: "relative",
          }}
        >
          {renderCellContent(cell)}
        </div>
      ))}
    </div>
  );
}

// Функция, возвращающая фон по умолчанию, если изображение не задано
function getTileColor(cell: Cell): string {
  switch (cell.tileCode) {
    case 48: // '0'
      return "#CCCCCC"; // светло-серый (проходимый)
    case 80: // 'P'
      return "#0000FF"; // синий (портал/старт)
    case 32: // пробел
      return "#333333"; // темно-серый (непроходимый)
    case 77: // 'M'
      return "#FF0000"; // красный (монстр)
    case 82: // 'R'
      return "#00AA00"; // зелёный (ресурс)
    case 112: // 'p'
      return "#02FEC0"; // для портала (настраивается)
    default:
      return "#952215"; // по умолчанию
  }
}

// Функция, которая возвращает JSX для содержимого клетки: изображение монстра или ресурса, если оно задано
function renderCellContent(cell: Cell) {
  if (cell.monster && cell.monster.image) {
    return (
      <img
        src={cell.monster.image}
        alt="monster"
        style={{ width: "100%", height: "100%", objectFit: "cover" }}
      />
    );
  }
  if (cell.resource && cell.resource.image) {
    return (
      <img
        src={cell.resource.image}
        alt="resource"
        style={{ width: "100%", height: "100%", objectFit: "cover" }}
      />
    );
  }
  // Если нет изображения, можно показать номер или оставить пустым
  return null;
}
