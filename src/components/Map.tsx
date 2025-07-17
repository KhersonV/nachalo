
//==================================
// src/components/Map.tsx
//==================================

"use client";

import React, { useMemo } from "react";
import { useSelector } from "react-redux";
import { Cell } from "@/types/GameTypes";
import { RootState } from "@/store";
import { buildPlayerMap } from "@/utils/buildPlayerMap";
import MapCell from "./MapCell";
import { PlayerState } from "@/types/GameTypes";

export interface MapProps {
  grid: Cell[];
  mapWidth: number;
  mapHeight: number;
  tileSize: number;
  gap: number;
  visionRange: number;
  playerPosition: { x: number; y: number };
  onCellClick?: (cell: Cell) => void;
}

function Map({
  grid,
  mapWidth,
  mapHeight,
  tileSize,
  gap,
  visionRange,
  playerPosition,
  onCellClick,
}: MapProps) {
  const players = useSelector((state: RootState) => state.game.players);

  const fullWidth = mapWidth * tileSize + (mapWidth - 1) * gap;
  const fullHeight = mapHeight * tileSize + (mapHeight - 1) * gap;

  // ✅ Мемоизированный playerMap
  const playerMap = useMemo(() => buildPlayerMap(players), [players]);

  // ✅ Мемоизированные видимые клетки с игроком
const cellsWithVisibility = useMemo(() => {
  if (!Array.isArray(grid) || grid.length === 0) return [];

  const result: { cell: Cell; visible: boolean; player: PlayerState | null }[] = [];

  for (const cell of grid) {
    const dx = Math.abs(cell.x - playerPosition.x);
    const dy = Math.abs(cell.y - playerPosition.y);
    const visible = dx <= visionRange && dy <= visionRange;

    const key = `${cell.x}-${cell.y}`;
    const player = visible ? playerMap.get(key) || null : null;

    result.push({ cell, visible, player });
  }

  return result;
}, [grid, playerPosition.x, playerPosition.y, visionRange, playerMap]);


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
      {cellsWithVisibility.map(({ cell, visible, player }) => (
        <MapCell
          key={`${cell.x}-${cell.y}`}
          cell={cell}
          visible={visible}
          playerInCell={player}
          tileSize={tileSize}
          onClick={onCellClick}
        />
      ))}
    </div>
  );
}

export default React.memo(Map);
