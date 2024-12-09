"use client";

import React, { useState, useEffect, useCallback } from "react";
import Tile from "./Tile";
import { resources, ResourceType } from "./resources/ResourceData";
import { usePlayer } from "./Player";
import "../styles/map.css";

type Cell = {
  id: number;
  x: number;
  y: number;
  terrain: string;
  resource: ResourceType | null;
};

type MapProps = {
  updateInventory: React.Dispatch<
    React.SetStateAction<
      Record<string, { count: number; image: string; description: string }>
    >
  >;
};

const mapWidth = 20;
const mapHeight = 20;
const terrains = ["water", "ground", "mountain", "forest", "ice"];

export default function Map({ updateInventory }: MapProps) {
  const { state: playerState, move } = usePlayer({
    position: { x: 0, y: 0 },
    energy: 100,
    level: 1,
    visionRange: 5,
    health: 100,
    attack: 10,
    defense: 5,
  });

  const generateResource = (terrain: string): ResourceType | null => {
    const availableResources = Object.values(resources).filter((res) =>
      res.terrains.includes(terrain)
    );
    if (availableResources.length === 0) return null;
    return availableResources[Math.floor(Math.random() * availableResources.length)];
  };

  const generateGrid = (width: number, height: number): Cell[] => {
    const total = width * height;
    return Array.from({ length: total }, (_, id) => {
      const x = id % width;
      const y = Math.floor(id / width);
      const terrain = terrains[Math.floor(Math.random() * terrains.length)];
      const resource = Math.random() < 0.15 ? generateResource(terrain) : null;
      return { id, x, y, terrain, resource };
    });
  };

  const [grid, setGrid] = useState<Cell[] | null>(null);

  // Генерация карты только на клиенте после монтирования
  useEffect(() => {
    const generated = generateGrid(mapWidth, mapHeight);
    setGrid(generated);
  }, []);

  const collectResource = useCallback(() => {
    if (!grid) return;
    const playerX = playerState.position.x;
    const playerY = playerState.position.y;

    const cellIndex = grid.findIndex((c) => c.x === playerX && c.y === playerY);
    if (cellIndex === -1) return;

    const cell = grid[cellIndex];

    if (cell.resource) {
      const { type, image, description } = cell.resource;
      updateInventory((prev) => {
        const newInventory = { ...prev };
        newInventory[type] = newInventory[type]
          ? { ...newInventory[type], count: newInventory[type].count + 1 }
          : { count: 1, image, description };
        return newInventory;
      });

      setGrid((prev) =>
        prev ? prev.map((c) => (c.id === cell.id ? { ...c, resource: null } : c)) : prev
      );
    }
  }, [grid, playerState.position, updateInventory]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (!grid) return; // Не двигаемся пока карта не загружена
      if (e.key === "ArrowUp") {
        move(0, -1, mapWidth, mapHeight);
      } else if (e.key === "ArrowDown") {
        move(0, 1, mapWidth, mapHeight);
      } else if (e.key === "ArrowLeft") {
        move(-1, 0, mapWidth, mapHeight);
      } else if (e.key === "ArrowRight") {
        move(1, 0, mapWidth, mapHeight);
      } else if (e.key === " ") {
        collectResource();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [move, collectResource, grid]);

  if (!grid) {
    return <div>Loading...</div>;
  }

  const { position, visionRange } = playerState;
  const startX = Math.max(position.x - visionRange, 0);
  const endX = Math.min(position.x + visionRange, mapWidth - 1);
  const startY = Math.max(position.y - visionRange, 0);
  const endY = Math.min(position.y + visionRange, mapHeight - 1);

  const visibleTiles = grid.filter((cell) =>
    cell.x >= startX && cell.x <= endX && cell.y >= startY && cell.y <= endY
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
      {visibleTiles.map((cell) => (
        <Tile
          key={cell.id}
          cell={cell}
          isPlayer={cell.x === position.x && cell.y === position.y}
        />
      ))}
    </div>
  );
}
