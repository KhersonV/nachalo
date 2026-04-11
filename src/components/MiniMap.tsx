"use client";

import React from "react";
import "../styles/minimap.css";

type MiniMapProps = {
  grid: { id: number; x: number; y: number; terrain: string; visited: boolean }[];
  playerPosition: { x: number; y: number };
  gridSize: number;
};

export default function MiniMap({ grid, playerPosition, gridSize }: MiniMapProps) {
  return (
    <div className="minimap">
      {grid.map((cell) => (
        <div
          key={cell.id}
          className={`minimap-cell ${cell.terrain} ${
            cell.visited ? "visited" : "unexplored"
          } ${cell.x === playerPosition.x && cell.y === playerPosition.y ? "player" : ""}`}
          style={{
            width: `${100 / gridSize}%`,
            height: `${100 / gridSize}%`,
          }}
        />
      ))}
    </div>
  );
}
