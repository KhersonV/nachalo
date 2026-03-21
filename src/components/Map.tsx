//==================================
// src/components/Map.tsx
//==================================

"use client";

import React, { useMemo, useRef } from "react";
import { Cell, PlayerState } from "@/types/GameTypes";
import MapCell from "./MapCell";
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
    players?: PlayerState[];
}

type CellVisibility = "visible" | "explored";

type RenderCell = {
    cell: Cell;
    visibility: CellVisibility;
    player: PlayerState | null;
};

function getCellIndex(x: number, y: number, mapWidth: number): number {
    return y * mapWidth + x;
}

function getCellAt(
    grid: Cell[],
    x: number,
    y: number,
    mapWidth: number,
    mapHeight: number,
): Cell | null {
    if (x < 0 || y < 0 || x >= mapWidth || y >= mapHeight) {
        return null;
    }

    return grid[getCellIndex(x, y, mapWidth)] ?? null;
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
    players = [],
}: MapProps) {
    const fullWidth = mapWidth * tileSize + (mapWidth - 1) * gap;
    const fullHeight = mapHeight * tileSize + (mapHeight - 1) * gap;
    const step = tileSize + gap;

    const playerMap = useMemo(() => {
        const map = new globalThis.Map<number, PlayerState>();

        for (const player of players) {
            const pos = player.position;
            if (!pos) continue;

            map.set(getCellIndex(pos.x, pos.y, mapWidth), player);
        }

        return map;
    }, [players, mapWidth]);

    // Track explored cells
    const exploredCellsRef = useRef<Set<number>>(new Set());
    const lastMapKeyRef = useRef<string>("");

    const cellsWithVisibility = useMemo<RenderCell[]>(() => {
        if (!Array.isArray(grid) || grid.length === 0) {
            return [];
        }

        const mapKey = `${mapWidth}x${mapHeight}`;
        if (mapKey !== lastMapKeyRef.current) {
            lastMapKeyRef.current = mapKey;
            exploredCellsRef.current = new Set<number>();
        }

        const result: RenderCell[] = [];
        const seenKeys = new Set<number>();

        // Player's visible area
        const visibleMinX = Math.max(0, playerPosition.x - sightRange);
        const visibleMaxX = Math.min(
            mapWidth - 1,
            playerPosition.x + sightRange,
        );
        const visibleMinY = Math.max(0, playerPosition.y - sightRange);
        const visibleMaxY = Math.min(
            mapHeight - 1,
            playerPosition.y + sightRange,
        );

        // Add visible cells and mark them as explored
        for (let y = visibleMinY; y <= visibleMaxY; y++) {
            for (let x = visibleMinX; x <= visibleMaxX; x++) {
                const cell = getCellAt(grid, x, y, mapWidth, mapHeight);
                if (!cell) continue;

                const key = getCellIndex(x, y, mapWidth);
                exploredCellsRef.current.add(key);
                seenKeys.add(key);

                result.push({
                    cell,
                    visibility: "visible",
                    player: playerMap.get(key) || null,
                });
            }
        }

        /**
         * IMPORTANT:
         * Do not render all explored cells of the entire map.
         * Render only a local window around the player.
         */
        const exploredBuffer = 3;

        const renderMinX = Math.max(0, visibleMinX - exploredBuffer);
        const renderMaxX = Math.min(mapWidth - 1, visibleMaxX + exploredBuffer);
        const renderMinY = Math.max(0, visibleMinY - exploredBuffer);
        const renderMaxY = Math.min(
            mapHeight - 1,
            visibleMaxY + exploredBuffer,
        );

        for (let y = renderMinY; y <= renderMaxY; y++) {
            for (let x = renderMinX; x <= renderMaxX; x++) {
                const key = getCellIndex(x, y, mapWidth);

                if (seenKeys.has(key)) {
                    continue;
                }

                if (!exploredCellsRef.current.has(key)) {
                    continue;
                }

                const cell = getCellAt(grid, x, y, mapWidth, mapHeight);
                if (!cell) continue;

                result.push({
                    cell,
                    visibility: "explored",
                    player: null,
                });
            }
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
                position: "relative",
            }}
        >
            {cellsWithVisibility.map(({ cell, visibility, player }) => {
                const cellKey = getCellIndex(cell.x, cell.y, mapWidth);

                return (
                    <div
                        key={cellKey}
                        style={{
                            position: "absolute",
                            left: cell.x * step,
                            top: cell.y * step,
                            width: tileSize,
                            height: tileSize,
                        }}
                    >
                        <MapCell
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
                    </div>
                );
            })}
        </div>
    );
}

export default React.memo(Map);
