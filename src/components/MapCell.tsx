//==================================
// src/components/MapCell.tsx
//==================================

import React from "react";
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
    players?: PlayerState[];
    startOwners?: Record<string, number>;
}

const IMAGE_STYLE: React.CSSProperties = {
    width: "100%",
    height: "100%",
    objectFit: "cover",
};

function MapCell({
    cell,
    playerInCell,
    visibility,
    tileSize,
    isCurrentPlayerCell = false,
    onClick,
    players,
    startOwners,
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
        background: getTileBackground(cell, players, startOwners),
        pointerEvents: isVisible ? "auto" : "none",
        cursor: isInteractive ? "pointer" : "default",
    };

    const tileClassName =
        `${styles.cell} ` +
        `${isVisible ? styles.visible : isExplored ? styles.explored : styles.unknown} ` +
        `${isInteractive ? styles.interactive : ""} ` +
        `${isCurrentPlayerCell ? styles.currentPlayerCell : ""} ` +
        `${playerInCell ? styles.hasPlayer : ""}`;

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

function getTileBackground(
    cell: Cell,
    players?: PlayerState[] | null,
    startOwners?: Record<string, number> | null,
): string {
    // If this cell is a start tile (tileCode 80 / 'P'), color by the original owner stored in startOwners
    if (cell.tileCode === 80) {
        const key = `${cell.x}:${cell.y}`;
        const ownerId = startOwners?.[key] ?? null;
        const owner = ownerId
            ? (players?.find((p) => p.user_id === ownerId) ?? null)
            : null;
        const groupId = owner?.group_id ?? null;

        if (groupId === 1)
            return "linear-gradient(155deg, #2f68bf 0%, #183a72 100%)";
        if (groupId === 2)
            return "linear-gradient(155deg, #a93939 0%, #7f2727 100%)";
        if (groupId === 3)
            return "linear-gradient(155deg, #f2c94c 0%, #c58f16 100%)";
    }

    // If this cell is a player base structure, try to color by owner's team (group_id)
    if (cell.structure_type === "base" && cell.structure_owner_user_id) {
        const ownerId = cell.structure_owner_user_id;
        const owner = players?.find((p) => p.user_id === ownerId) ?? null;
        const groupId = owner?.group_id ?? null;
        if (groupId === 1)
            return "linear-gradient(155deg, #2f68bf 0%, #183a72 100%)";
        if (groupId === 2)
            return "linear-gradient(155deg, #a93939 0%, #7f2727 100%)";
        if (groupId === 3)
            return "linear-gradient(155deg, #f2c94c 0%, #c58f16 100%)";
    }

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

function areEqual(prev: MapCellProps, next: MapCellProps) {
    const prevCell = prev.cell;
    const nextCell = next.cell;

    return (
        prev.visibility === next.visibility &&
        prev.tileSize === next.tileSize &&
        prev.isCurrentPlayerCell === next.isCurrentPlayerCell &&
        prev.onClick === next.onClick &&
        prev.players === next.players &&
        prev.startOwners === next.startOwners &&
        !!prev.playerInCell === !!next.playerInCell &&
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
