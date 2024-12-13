// Tile.tsx
"use client";

import React, { useState } from "react";
import "../styles/tile.css";
import { terrainData } from "../logic/terrainData";
import { useGameContext } from "./GameContext";
import { getResourceImage } from "./resources/ResourceData";
import { Cell } from "../logic/types";

type TileProps = {
  cell: Cell;
  playersOnTile: number[];
};

export default function Tile({ cell, playersOnTile }: TileProps) {
  const { state, setState } = useGameContext();
  const [hasResource, setHasResource] = useState(cell.resource !== null);

  const terrainInfo = terrainData[cell.terrain] || terrainData["ground"];

  // Сначала определяем переменную для изображения тайла
  let tileBackground = terrainInfo.image;
  if (hasResource && cell.resource) {
    tileBackground = getResourceImage(cell.resource, cell.terrain);
  }

  const handleCollectResource = () => {
    // Сохраняем в переменную, чтобы TS понял, что resource не null после проверки
    const resource = cell.resource;
    if (!resource) return;

    setState((prev) => {
      const activePlayer = prev.players[prev.currentPlayerIndex];
      const updatedInventory = { ...activePlayer.inventory };
      const resourceType = resource.type;

      if (updatedInventory[resourceType]) {
        updatedInventory[resourceType].count += 1;
      } else {
        updatedInventory[resourceType] = {
          count: 1,
          image: getResourceImage(resource, cell.terrain),
          description: resource.description,
        };
      }

      const updatedPlayer = { ...activePlayer, inventory: updatedInventory };
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

    setHasResource(false);
  };

  const monster = cell.monster;
  const isPortal = cell.isPortal;
  const resource = cell.resource;

  return (
    <div
      className="tile"
      style={{
        backgroundImage: `url(${tileBackground})`,
        backgroundSize: "cover",
      }}
      // onClick={handleCollectResource}
    >
      {isPortal && (
        <div className="portal-indicator">
          <img src="/portal.webp" alt="Portal" style={{ width: "50px", height: "50px" }} />
        </div>
      )}
      {monster && (
        <div className="monster-indicator">
          <img
            src={monster.image[cell.terrain]}
            alt={monster.name}
            style={{ width: "50px", height: "50px" }}
          />
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
      {hasResource && resource && (
        <div className="resource">
          {/* <img
            src={getResourceImage(resource, cell.terrain)}
            alt={resource.type}
            className="resource-image"
          /> */}
          <div className="tooltip">
            <p>{resource.type}</p>
            <p>{resource.description}</p>
          </div>
        </div>
      )}
    </div>
  );
}
