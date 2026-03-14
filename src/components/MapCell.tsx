import React, { useMemo, useRef, useState } from "react";
import { Cell, PlayerState } from "@/types/GameTypes";
import styles from "@/styles/Map.module.css";

type Visibility = "visible" | "explored" | "unknown";

interface MapCellProps {
    cell: Cell;
    playerInCell: PlayerState | null;
    visibility: Visibility;
    tileSize: number;
    isCurrentPlayerCell?: boolean;
    onClick?: (cell: Cell) => void;
}

function MapCell({
    cell,
    playerInCell,
    visibility,
    tileSize,
    isCurrentPlayerCell = false,
    onClick,
}: MapCellProps) {
    const [hovered, setHovered] = useState(false);
    const [showTooltip, setShowTooltip] = useState(false);
    const tooltipTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    const getCellTooltip = () => {
        if (cell.monster)
            return `👹 Монстр: ${cell.monster.name}\n❤️ ${cell.monster.health}\n⚔️ ${cell.monster.attack}\n🛡 ${cell.monster.defense}`;
        if (cell.resource) return `⛏ Ресурс: ${cell.resource.type}`;
        if (cell.barbel) return `🪵 Бочка`;
        if (cell.isPortal) return `🌀 Портал`;
        if (cell.isPlayer && playerInCell) {
            return `🧍 Игрок: ${playerInCell.name}\n❤️ ${playerInCell.health}\n⚔️ ${playerInCell.attack}\n🛡 ${playerInCell.defense}`;
        }
        return `${cell.x} ${cell.y} `;
    };

    const imageStyle = {
        width: "100%",
        height: "100%",
        objectFit: "cover",
    } as React.CSSProperties;

    const tileStyle = useMemo<React.CSSProperties>(() => {
        return {
            width: tileSize,
            height: tileSize,
            background: getTileBackground(cell),
            pointerEvents: visibility === "visible" ? "auto" : "none",
            cursor: visibility === "visible" && onClick ? "pointer" : "default",
        };
    }, [cell, visibility, tileSize, onClick]);

    const tileClassName = [
        styles.cell,
        visibility === "visible"
            ? styles.visible
            : visibility === "explored"
              ? styles.explored
              : styles.unknown,
        visibility === "visible" && onClick ? styles.interactive : "",
        isCurrentPlayerCell ? styles.currentPlayerCell : "",
        playerInCell ? styles.hasPlayer : "",
    ]
        .filter(Boolean)
        .join(" ");

    const renderCellContent = () => {
        if (cell.monster?.image) {
            return (
                <img
                    src={cell.monster.image}
                    alt="monster"
                    className={styles.image}
                    style={imageStyle}
                />
            );
        }
        if (cell.resource?.image) {
            return (
                <img
                    src={cell.resource.image}
                    alt="resource"
                    className={styles.image}
                    style={imageStyle}
                />
            );
        }
        if (cell.barbel?.image) {
            return (
                <img
                    src={cell.barbel.image}
                    alt="barrel"
                    className={styles.image}
                    style={imageStyle}
                />
            );
        }
        if (cell.isPortal) {
            return <span className={styles.symbol}>🌀</span>;
        }
        if (cell.monster) {
            return <span className={styles.symbol}>👹</span>;
        }
        if (cell.resource) {
            return <span className={styles.symbol}>⛏</span>;
        }
        if (cell.barbel) {
            return <span className={styles.symbol}>🪵</span>;
        }
        return null;
    };

    return (
        <div
            key={`${cell.x}-${cell.y}`}
            className={tileClassName}
            style={tileStyle}
            onClick={() => visibility === "visible" && onClick?.(cell)}
            onMouseEnter={() => {
                if (tooltipTimerRef.current) {
                    clearTimeout(tooltipTimerRef.current);
                }
                tooltipTimerRef.current = setTimeout(() => {
                    setHovered(true);
                    setShowTooltip(true);
                }, 450);
            }}
            onMouseLeave={() => {
                if (tooltipTimerRef.current) {
                    clearTimeout(tooltipTimerRef.current);
                    tooltipTimerRef.current = null;
                }
                setShowTooltip(false);
                setHovered(false);
            }}
        >
            {/* Энтити (монстры, ресурсы) только в зоне видимости */}
            {visibility === "visible" && renderCellContent()}
            {visibility === "visible" && (
                <span className={styles.coords}>{`${cell.x}:${cell.y}`}</span>
            )}

            {visibility === "visible" && showTooltip && hovered && (
                <div className={styles.tooltip}>{getCellTooltip()}</div>
            )}
        </div>
    );
}

function getTileBackground(cell: Cell): string {
    switch (cell.tileCode) {
        case 48:
            return "linear-gradient(155deg, #9da5ad 0%, #7b858f 100%)";
        case 80:
            return "linear-gradient(155deg, #2f68bf 0%, #183a72 100%)";
        case 32:
            return "linear-gradient(155deg, #3f4954 0%, #2a3139 100%)";
        case 77:
            return "linear-gradient(155deg, #a93939 0%, #7f2727 100%)";
        case 82:
            return "linear-gradient(155deg, #2f8d64 0%, #1f6a4a 100%)";
        case 112:
            return "linear-gradient(155deg, #45c7b0 0%, #1b8f8d 100%)";
        case 66:
            return "linear-gradient(155deg, #cb8a45 0%, #8f5f2e 100%)";
        default:
            return "linear-gradient(155deg, #8a4b42 0%, #6d362f 100%)";
    }
}

export default React.memo(MapCell);
