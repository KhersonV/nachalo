"use client";

import React, { useState } from "react";
import "../styles/tile.css";
import { terrainData } from "../logic/terrainData";
import { useGameContext } from "./GameContext";
import { getResourceImage, ResourceType } from "./resources/ResourceData";

type TileProps = {
  cell: {
    id: number;
    x: number;
    y: number;
    terrain: string;
    resource: ResourceType | null;
    isBarrel?: boolean;
    isPortal?: boolean;
    monster?: {
      name: string;
      type: "aggressive" | "neutral";
      hp: number;
      maxHp: number;
      attack: number;
      defense: number;
      vision: number;
      image: string;
    };
  };
  playersOnTile: number[];
};

export default function Tile({ cell, playersOnTile }: TileProps) {
  const { state, setState } = useGameContext();
  const [hasResource, setHasResource] = useState(cell.resource !== null);

  // Фон тайла
  const terrainInfo = terrainData[cell.terrain] || terrainData["ground"];
  const tileBackground = hasResource && cell.resource
    ? getResourceImage(cell.resource, cell.terrain)
    : terrainInfo.image;

  const handleCollectResource = () => {
    if (!cell.resource) return;

    // Обновляем состояние игрока и убираем ресурс
    setState((prev) => {
      const activePlayer = prev.players[prev.currentPlayerIndex];
      const updatedInventory = { ...activePlayer.inventory };
      const resourceType = cell.resource!.type;

      // Увеличиваем количество ресурса в инвентаре игрока
      if (updatedInventory[resourceType]) {
        updatedInventory[resourceType].count += 1;
      } else {
        updatedInventory[resourceType] = {
          count: 1,
          image: getResourceImage(cell.resource, cell.terrain),
          description: cell.resource.description,
        };
      }

      // Обновляем игрока
      const updatedPlayer = {
        ...activePlayer,
        inventory: updatedInventory,
      };

      // Обновляем сетку, убирая ресурс с клетки
      const updatedGrid = prev.grid!.map((c) =>
        c.id === cell.id ? { ...c, resource: null } : c
      );

      return {
        ...prev,
        players: prev.players.map((p, i) =>
          i === prev.currentPlayerIndex ? updatedPlayer : p
        ),
        grid: updatedGrid,
      };
    });

    // Убираем ресурс с текущего тайла
    setHasResource(false);
  };

  return (
    <div
      className="tile"
      style={{
        backgroundImage: `url(${tileBackground})`,
        backgroundSize: "cover",
      }}
      onClick={handleCollectResource} // Собираем ресурс при клике
    >
      {cell.isPortal && (
        <div className="portal-indicator">
          <img src="/portal.webp" alt="Portal" style={{ width: "50px", height: "50px" }} />
        </div>
      )}
      {cell.isBarrel && (
        <div className="barrel-indicator">
          <img src="/barrel.webp" alt="Barrel" style={{ width: "50px", height: "50px" }} />
        </div>
      )}
      {cell.monster && (
        <div className="monster-indicator">
          <img src={cell.monster.image} alt={cell.monster.name} style={{ width: "50px", height: "50px" }} />
        </div>
      )}
      {playersOnTile.length > 0 && (
        <div className="players-on-tile">
          {playersOnTile.map((playerIndex) => {
            const player = state.players.find((p) => p.id === playerIndex);
            return player ? (
              <img
                key={player.id}
                src={player.image}
                alt={player.name}
                style={{ width: "50px", height: "50px" }}
              />
            ) : null;
          })}
        </div>
      )}
      {hasResource && cell.resource && (
        <div className="resource">
          <img
            src={getResourceImage(cell.resource, cell.terrain)}
            alt={cell.resource.type}
            className="resource-image"
          />
          <div className="tooltip">
            <p>{cell.resource.type}</p>
            <p>{cell.resource.description}</p>
          </div>
        </div>
      )}
    </div>
  );
}
