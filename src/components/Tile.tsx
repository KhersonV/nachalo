"use client";

import React from "react";
import "../styles/tile.css";

type Resource = {
  type: string;
  image: string;
  description: string;
};

type TileProps = {
  cell: {
    id: number;
    x: number;
    y: number;
    terrain: string;
    resource: Resource | null;
  };
  isPlayer: boolean;
};

export default function Tile({ cell, isPlayer }: TileProps) {
  return (
    <div className={`tile ${cell.terrain}`}>
      {isPlayer && <div className="player" />}
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
