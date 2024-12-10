"use client";

import React from "react";
import "../styles/tile.css";
import { terrainData } from "../logic/terrainData";
import { useGameContext } from "./GameContext";

export type Resource = {
  type: string;
  image: string;
  description: string;
};

export type Cell = {
  id: number;
  x: number;
  y: number;
  terrain: string;
  resource: Resource | null;
  isBarrel?: boolean;
  isPortal?: boolean;
  monster?: {
    name: string;
    type: 'aggressive'|'neutral';
    hp: number;
    maxHp: number;
    attack: number;
    defense: number;
    vision: number;
    image: string;
  };
};

type TileProps = {
  cell: Cell;
  playersOnTile: number[];
};

export default function Tile({ cell, playersOnTile }: TileProps) {
  // Фон берем из terrainData
  const terrainInfo = terrainData[cell.terrain] || terrainData['ground'];
  const { state } = useGameContext();

  return (
    <div className="tile" style={{
      backgroundImage: `url(${terrainInfo.image})`,
      backgroundSize: 'cover'
    }}>
      {cell.isPortal && <div className="portal-indicator"><img src={'portal.webp'} alt={'Portal'} style={{width:'50px',height:'50px'}}/></div>}
      {cell.isBarrel && <div className="barrel-indicator">
        <img src={'barrel.webp'} alt={'Barrel'} style={{width:'50px',height:'50px'}}/></div>}
      {cell.monster && (
        <div className="monster-indicator">
          <img src={cell.monster.image} alt={cell.monster.name} style={{width:'50px',height:'50px'}}/>
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
          <img src={cell.resource.image} alt={cell.resource.type} className="resource-image" />
          <div className="tooltip">
            <p>{cell.resource.type}</p>
            <p>{cell.resource.description}</p>
          </div>
        </div>
      )}
    </div>
  );
}
