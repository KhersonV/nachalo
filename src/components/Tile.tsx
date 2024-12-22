// src/components/Tile.tsx

"use client";

import React, { useState, useEffect } from "react";
import "../styles/tile.css";
import { terrainData } from "../logic/allData";
import { useGameContext } from "./GameContext";
import { getResourceImage } from "../logic/allData";
import { Cell } from "../logic/types";


type TileProps = {
  cell: Cell;
  playersOnTile: number[];
};

export default function Tile({ cell, playersOnTile }: TileProps) {
  const { state, dispatch } = useGameContext();
  const [tileBackground, setTileBackground] = useState<string>(terrainData[cell.terrain].image);

  useEffect(() => {
    if (cell.monster) {
      setTileBackground(cell.monster.image[cell.terrain]);
    } else {
      setTileBackground(terrainData[cell.terrain].image);
    }
  }, [cell.monster, cell.terrain]);

  const handleTileClick = () => {
    const playerId = state.players[state.currentPlayerIndex]?.id;
    if (!playerId) return;

    // Взаимодействие с ресурсом
    if (cell.resource) {
      dispatch({
        type: 'COLLECT_RESOURCE',
        payload: {
          playerId,
          resourceType: cell.resource.type,
          cellId: cell.id
        },
      });
    }

    // Взаимодействие с порталом
    if (cell.isPortal) {
      dispatch({ type: 'TRY_EXIT_PORTAL', payload: { playerId } });
    }
  };

  return (
    <div
      className="tile"
      style={{
        backgroundImage: `url(${tileBackground})`,
        backgroundSize: "cover",
      }}
      onClick={handleTileClick}
    >
      {cell.isPortal && (
        <div className="portal-indicator">
          <img src="/portal.webp" alt="Portal" style={{ width: "50px", height: "50px" }} />
        </div>
      )}
      {playersOnTile.length > 0 && (
        <div className="players-on-tile">
          {playersOnTile.map((playerId) => {
            const player = state.players.find((p) => p.id === playerId);
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
