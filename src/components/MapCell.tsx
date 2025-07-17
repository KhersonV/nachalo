import React, { useState } from "react";
import { Cell, PlayerState } from "@/types/GameTypes";

interface MapCellProps {
  cell: Cell;
  playerInCell: PlayerState | null;
  visible: boolean;
  tileSize: number;
  onClick?: (cell: Cell) => void;
}

function MapCell({ cell, playerInCell, visible, tileSize, onClick }: MapCellProps) {
  const [hovered, setHovered] = useState(false);
  const [showTooltip, setShowTooltip] = useState(false);
  let tooltipTimer: NodeJS.Timeout;

  const getCellTooltip = () => {
    if (cell.monster) return `👹 Монстр: ${cell.monster.name}\n❤️ ${cell.monster.health}\n⚔️ ${cell.monster.attack}\n🛡 ${cell.monster.defense}`;
    if (cell.resource) return `⛏ Ресурс: ${cell.resource.type}`;
    if (cell.barbel) return `🪵 Бочка`;
    if (cell.isPortal) return `🌀 Портал`;
    if (cell.isPlayer && playerInCell) {
      return `🧍 Игрок: ${playerInCell.name}\n❤️ ${playerInCell.health}\n⚔️ ${playerInCell.attack}\n🛡 ${playerInCell.defense}`;
    }
    return `${cell.x} ${cell.y} `;
  };

  const renderCellContent = () => {
    if (cell.monster?.image) {
      return (
        <img
          src={cell.monster.image}
          alt="monster"
          style={imageStyle}
        />
      );
    }
    if (cell.resource?.image) {
      return (
        <img
          src={cell.resource.image}
          alt="resource"
          style={imageStyle}
        />
      );
    }
    if (cell.barbel?.image) {
      return (
        <img
          src={cell.barbel.image}
          alt="barrel"
          style={imageStyle}
        />
      );
    }
    return null;
  };

  const imageStyle = {
    width: "100%",
    height: "100%",
    objectFit: "cover",
  } as React.CSSProperties;

  return (
    <div
      key={`${cell.x}-${cell.y}`}
      style={{
        width: tileSize,
        height: tileSize,
        backgroundColor: getTileColor(cell),
        opacity: visible ? 1 : 0,
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
        position: "relative",
        pointerEvents: visible ? "auto" : "none",
        cursor: visible && onClick ? "pointer" : "default",
      }}
      onClick={() => visible && onClick?.(cell)}
      onMouseEnter={() => {
        tooltipTimer = setTimeout(() => {
          setHovered(true);
          setShowTooltip(true);
        }, 2000);
      }}
      onMouseLeave={() => {
        clearTimeout(tooltipTimer);
        setShowTooltip(false);
        setHovered(false);
      }}
    >
      {renderCellContent()}

      {showTooltip && hovered && (
        <div
          style={{
            position: "absolute",
            bottom: "100%",
            left: "50%",
            transform: "translateX(-50%)",
            background: "#222",
            color: "#fff",
            padding: "4px 6px",
            fontSize: "12px",
            borderRadius: "4px",
            whiteSpace: "pre-line",
            zIndex: 100,
          }}
        >
          {getCellTooltip()}
        </div>
      )}
    </div>
  );
}

function getTileColor(cell: Cell): string {
  switch (cell.tileCode) {
    case 48: return "#CCCCCC";
    case 80: return "#0000FF";
    case 32: return "#333333";
    case 77: return "#FF0000";
    case 82: return "#00AA00";
    case 112: return "#02FEC0";
    case 66: return "#FFA500";
    default: return "#952215";
  }
}

export default React.memo(MapCell);
