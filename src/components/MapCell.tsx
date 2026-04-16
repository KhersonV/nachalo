//==================================
// src/components/MapCell.tsx
//==================================

import React from "react";
import { Cell } from "@/types/GameTypes";
import styles from "@/styles/Map.module.css";

type Visibility = "visible" | "explored" | "unknown";

interface MapCellProps {
    cell: Cell;
    hasPlayer: boolean;
    background: string;
    visibility: Visibility;
    tileSize: number;
    isCurrentPlayerCell?: boolean;
    onClick?: (cell: Cell) => void;
}

const IMAGE_STYLE: React.CSSProperties = {
    width: "100%",
    height: "100%",
    objectFit: "cover",
};

function MapCell({
    cell,
    hasPlayer,
    background,
    visibility,
    tileSize,
    isCurrentPlayerCell = false,
    onClick,
}: MapCellProps) {
    const isVisible = visibility === "visible";
    const isExplored = visibility === "explored";
    const isInteractive = isVisible && !!onClick;

    const structureType = cell.structure_type;
    const structureImage = (cell as any).structure_image as string | undefined;
    const isUnderConstruction = cell.is_under_construction;
    const monsterImage = cell.monster?.image;
    const resourceImage = cell.resource?.image;
    const barrelImage = cell.barbel?.image;
    const hasMonster = !!cell.monster;
    const hasResource = !!cell.resource;
    const hasBarrel = !!cell.barbel;
    const isPortal = !!cell.isPortal;

    const tileStyle: React.CSSProperties = {
        width: tileSize,
        height: tileSize,
        background,
        pointerEvents: isVisible ? "auto" : "none",
        cursor: isInteractive ? "pointer" : "default",
    };

    const tileClassName =
        `${styles.cell} ` +
        `${isVisible ? styles.visible : isExplored ? styles.explored : styles.unknown} ` +
        `${isInteractive ? styles.interactive : ""} ` +
        `${isCurrentPlayerCell ? styles.currentPlayerCell : ""} ` +
        `${hasPlayer ? styles.hasPlayer : ""}`;

    let cellContent: React.ReactNode = null;

    if (isVisible) {
        if (structureType) {
            if (structureImage && !isUnderConstruction) {
                cellContent = (
                    <img
                        src={structureImage}
                        alt={structureType}
                        className={styles.image}
                        style={IMAGE_STYLE}
                    />
                );
            } else {
                const symbol =
                    structureType === "scout_tower"
                        ? "🗼"
                        : structureType === "turret"
                          ? "🔫"
                          : "🧱";

                cellContent = (
                    <span className={styles.symbol}>
                        {isUnderConstruction ? "🚧" : symbol}
                    </span>
                );
            }
        } else if (monsterImage) {
            cellContent = (
                <img
                    src={monsterImage}
                    alt="monster"
                    className={styles.image}
                    style={IMAGE_STYLE}
                />
            );
        } else if (resourceImage) {
            cellContent = (
                <img
                    src={resourceImage}
                    alt="resource"
                    className={styles.image}
                    style={IMAGE_STYLE}
                />
            );
        } else if (barrelImage) {
            cellContent = (
                <img
                    src={barrelImage}
                    alt="barrel"
                    className={styles.image}
                    style={IMAGE_STYLE}
                />
            );
        } else if (isPortal) {
            cellContent = (
                <img
                    src="/portal.png"
                    alt="portal"
                    className={styles.image}
                    style={IMAGE_STYLE}
                />
            );
        } else if (hasMonster) {
            cellContent = <span className={styles.symbol}>👹</span>;
        } else if (hasResource) {
            cellContent = <span className={styles.symbol}>⛏</span>;
        } else if (hasBarrel) {
            cellContent = <span className={styles.symbol}>🪵</span>;
        }
    }

    const handleClick = () => {
        if (isVisible) {
            onClick?.(cell);
        }
    };

    return (
        <div className={tileClassName} style={tileStyle} onClick={handleClick}>
            {cellContent}

            {isVisible && (
                <span className={styles.coords}>{`${cell.x}:${cell.y}`}</span>
            )}
        </div>
    );
}

function areEqual(prev: MapCellProps, next: MapCellProps) {
    const prevCell = prev.cell;
    const nextCell = next.cell;

    return (
        prev.hasPlayer === next.hasPlayer &&
        prev.background === next.background &&
        prev.visibility === next.visibility &&
        prev.tileSize === next.tileSize &&
        prev.isCurrentPlayerCell === next.isCurrentPlayerCell &&
        prev.onClick === next.onClick &&
        prevCell.x === nextCell.x &&
        prevCell.y === nextCell.y &&
        prevCell.tileCode === nextCell.tileCode &&
        prevCell.isPortal === nextCell.isPortal &&
        prevCell.structure_type === nextCell.structure_type &&
        prevCell.is_under_construction === nextCell.is_under_construction &&
        prevCell.monster === nextCell.monster &&
        prevCell.resource === nextCell.resource &&
        prevCell.barbel === nextCell.barbel &&
        prevCell.monster?.image === nextCell.monster?.image &&
        prevCell.resource?.image === nextCell.resource?.image &&
        prevCell.barbel?.image === nextCell.barbel?.image
    );
}

export default React.memo(MapCell, areEqual);
