// Tile.tsx
"use client";

import React, { useState, useEffect } from "react";
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
  const [tileBackground, setTileBackground] = useState<string>(terrainData[cell.terrain].image);

  useEffect(() => {
    // Если на клетке есть монстр, устанавливаем фон с изображением монстра
    if (cell.monster) {
      setTileBackground(cell.monster.image[cell.terrain]);
    } else {
      // Если монстр убит или отсутствует, возвращаем фон террейна
      setTileBackground(terrainData[cell.terrain].image);
    }
  }, [cell.monster, cell.terrain]);

  

  return (
    <div
      className="tile"
      style={{
        backgroundImage: `url(${tileBackground})`,
        backgroundSize: "cover",
      }}
    >
      {cell.isPortal && (
        <div className="portal-indicator">
          <img src="/portal.webp" alt="Portal" style={{ width: "50px", height: "50px" }} />
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
      {cell.resource && (
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
