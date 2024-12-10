"use client";

import React from "react";
import "../styles/tile.css";

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
  isMonster?: boolean;
};

type TileProps = {
  cell: Cell;
  playersOnTile: number[]; // индексы игроков на клетке
};

export default function Tile({ cell, playersOnTile }: TileProps) {
  let extraClass = "";
  if (cell.isPortal) extraClass += " portal";
  if (cell.isBarrel) extraClass += " barrel";
  if (cell.isMonster) extraClass += " monster";

  return (
    <div className={`tile ${cell.terrain}${extraClass}`}>
      {playersOnTile.length > 0 && (
        <div className="players-on-tile">
          {playersOnTile.map((playerIndex) => (
            <div key={playerIndex} className={`player-circle player-circle-${playerIndex}`}></div>
          ))}
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
