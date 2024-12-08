"use client";

import React, { useState, useEffect } from "react";
import Tile from "./Tile";
import Inventory from "./Inventory";
import { resources, ResourceType } from "./resources/ResourceData";
import { usePlayerMovement } from "./Player";
import "../styles/map.css";

type Position = { x: number; y: number };
type Cell = { id: number; x: number; y: number; terrain: string; resource: ResourceType | null };

export default function Map() {
  const gridSize = 10;
  const initialPosition: Position = { x: 0, y: 0 };

  const [grid, setGrid] = useState<Cell[]>([]);
  const [playerPosition, setPlayerPosition] = useState<Position>(initialPosition);
  const [inventory, setInventory] = useState<Record<string, { count: number; image: string; description: string }>>({});
  const [isInventoryOpen, setIsInventoryOpen] = useState(false);

  const movePlayer = usePlayerMovement(gridSize, setPlayerPosition);

  useEffect(() => {
    setGrid(generateGrid(gridSize));
  }, []);

  const generateGrid = (gridSize: number): Cell[] => {
    const terrains = ["water", "ground", "mountain", "forest", "ice"];
    return Array.from({ length: gridSize * gridSize }, (_, id) => {
      const x = id % gridSize;
      const y = Math.floor(id / gridSize);
      const terrain = terrains[Math.floor(Math.random() * terrains.length)];
      const resource = generateResource(terrain);
      return { id, x, y, terrain, resource };
    });
  };

  const generateResource = (terrain: string): ResourceType | null => {
    if (Math.random() > 0.15) return null;

    const availableResources = Object.values(resources).filter((res) =>
      res.terrains.includes(terrain)
    );
    if (availableResources.length === 0) return null;

    const totalRarity = availableResources.reduce((sum, res) => sum + res.rarity, 0);
    const rand = Math.random() * totalRarity;
    let cumulativeRarity = 0;

    for (const resource of availableResources) {
      cumulativeRarity += resource.rarity;
      if (rand < cumulativeRarity) return resource;
    }
    return null;
  };

  const collectResource = () => {
    const cellIndex = grid.findIndex(
      (cell) => cell.x === playerPosition.x && cell.y === playerPosition.y
    );
    if (cellIndex !== -1 && grid[cellIndex].resource) {
      const resource = grid[cellIndex].resource;
      setGrid((prev) =>
        prev.map((cell, index) =>
          index === cellIndex ? { ...cell, resource: null } : cell
        )
      );
      setInventory((prev) => ({
        ...prev,
        [resource.type]: {
          count: (prev[resource.type]?.count || 0) + 1,
          image: resource.image,
          description: resource.description,
        },
      }));
    }
  };

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      switch (e.key) {
        case "ArrowUp":
          movePlayer(0, -1);
          break;
        case "ArrowDown":
          movePlayer(0, 1);
          break;
        case "ArrowLeft":
          movePlayer(-1, 0);
          break;
        case "ArrowRight":
          movePlayer(1, 0);
          break;
        case " ":
          e.preventDefault(); // Предотвращает скроллинг страницы
          collectResource();
          break;
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [movePlayer, playerPosition, grid]);

  const toggleInventory = () => setIsInventoryOpen((prev) => !prev);

  return (
    <div className="map-container">
      <button className="inventory-toggle" onClick={toggleInventory}>
        {isInventoryOpen ? "Закрыть инвентарь" : "Открыть инвентарь"}
      </button>
      {isInventoryOpen && <Inventory items={inventory} onClose={toggleInventory} />}
      <div className="map">
        {grid.map((cell) => (
          <Tile
            key={cell.id}
            x={cell.x}
            y={cell.y}
            terrain={cell.terrain}
            resource={cell.resource}
            onCollectResource={collectResource} 
            isPlayerHere={cell.x === playerPosition.x && cell.y === playerPosition.y}
          />
        ))}
      </div>
    </div>
  );
}
