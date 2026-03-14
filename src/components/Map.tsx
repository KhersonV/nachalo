//==================================
// src/components/Map.tsx
//==================================

"use client";

import React, { useMemo, useRef } from "react";
import { useSelector } from "react-redux";
import { Cell } from "@/types/GameTypes";
import { RootState } from "@/store";
import { buildPlayerMap } from "@/utils/buildPlayerMap";
import MapCell from "./MapCell";
import { PlayerState } from "@/types/GameTypes";
import styles from "@/styles/Map.module.css";

export interface MapProps {
    grid: Cell[];
    mapWidth: number;
    mapHeight: number;
    tileSize: number;
    gap: number;
    sightRange: number;
    playerPosition: { x: number; y: number };
    onCellClick?: (cell: Cell) => void;
}

function Map({
    grid,
    mapWidth,
    mapHeight,
    tileSize,
    gap,
    sightRange,
    playerPosition,
    onCellClick,
}: MapProps) {
    const players = useSelector((state: RootState) => state.game.players);

    const fullWidth = mapWidth * tileSize + (mapWidth - 1) * gap;
    const fullHeight = mapHeight * tileSize + (mapHeight - 1) * gap;

    // ✅ Мемоизированный playerMap
    const playerMap = useMemo(() => buildPlayerMap(players), [players]);

    // Fog-of-war: запоминаем исследованные клетки (не сбрасываются при ходе)
    const exploredCellsRef = useRef<Set<string>>(new Set());
    const lastMapKeyRef = useRef<string>("");

    // ✅ Мемоизированные клетки с трёхуровневой видимостью: visible / explored / unknown
    const cellsWithVisibility = useMemo(() => {
        if (!Array.isArray(grid) || grid.length === 0) return [];

        // Сброс при смене карты (новый матч / другие размеры)
        const mapKey = `${mapWidth}x${mapHeight}`;
        if (mapKey !== lastMapKeyRef.current) {
            lastMapKeyRef.current = mapKey;
            exploredCellsRef.current = new Set<string>();
        }

        const result: {
            cell: Cell;
            visibility: "visible" | "explored" | "unknown";
            player: PlayerState | null;
        }[] = [];

        for (const cell of grid) {
            const dx = Math.abs(cell.x - playerPosition.x);
            const dy = Math.abs(cell.y - playerPosition.y);
            const inVision = dx <= sightRange && dy <= sightRange;
            const key = `${cell.x}-${cell.y}`;

            if (inVision) exploredCellsRef.current.add(key);

            const visibility: "visible" | "explored" | "unknown" = inVision
                ? "visible"
                : exploredCellsRef.current.has(key)
                  ? "explored"
                  : "unknown";

            result.push({
                cell,
                visibility,
                player: inVision ? playerMap.get(key) || null : null,
            });
        }

        return result;
    }, [
        grid,
        mapWidth,
        mapHeight,
        playerPosition.x,
        playerPosition.y,
        sightRange,
        playerMap,
    ]);

    return (
        <div
            className={styles.mapGrid}
            style={{
                width: `${fullWidth}px`,
                height: `${fullHeight}px`,
                gridTemplateColumns: `repeat(${mapWidth}, ${tileSize}px)`,
                gridTemplateRows: `repeat(${mapHeight}, ${tileSize}px)`,
                gap: `${gap}px`,
            }}
        >
            {cellsWithVisibility.map(({ cell, visibility, player }) => (
                <MapCell
                    key={`${cell.x}-${cell.y}`}
                    cell={cell}
                    visibility={visibility}
                    playerInCell={player}
                    tileSize={tileSize}
                    isCurrentPlayerCell={
                        cell.x === playerPosition.x &&
                        cell.y === playerPosition.y
                    }
                    onClick={onCellClick}
                />
            ))}
        </div>
    );
}

export default React.memo(Map);
