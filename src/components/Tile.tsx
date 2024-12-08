import React, { useState } from "react";
import { ResourceType } from "./resources/ResourceData";
import "../styles/tile.css";

type TileProps = {
  x: number;
  y: number;
  terrain: string;
  resource: ResourceType | null;
  isPlayerHere: boolean;
  onCollectResource: () => void;
};

export default function Tile({
  terrain,
  resource,
  isPlayerHere,
  onCollectResource,
}: TileProps) {
  const [showTooltip, setShowTooltip] = useState(false);

  return (
    <div
      className={`tile ${terrain}`}
      onMouseEnter={() => setShowTooltip(true)}
      onMouseLeave={() => setShowTooltip(false)}
    >
      {isPlayerHere && <div className="player" />}
      {resource && (
        <div className="resource" onClick={onCollectResource}>
          <img src={resource.image} alt={resource.type} className="resource-image" />
          {showTooltip && (
            <div className="tooltip">
              <strong>{resource.type}</strong>
              <p>{resource.description}</p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
