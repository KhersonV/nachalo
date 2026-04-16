//==================================
// src/components/Map.tsx
//==================================

"use client";

import React, { useEffect, useMemo, useRef } from "react";
import { Cell } from "@/types/GameTypes";
import {
    buildExplorationStorageKey,
    getCellIndex,
    loadExploredCells,
    persistExploredCells,
} from "@/utils/fogOfWar";
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
    startOwners?: Record<string, number>;
    occupiedCellIndices?: ReadonlySet<number>;
    ownerGroupByUserId?: Readonly<Record<number, number>>;
    explorationStorageKey?: string;
    renderCenterPosition?: { x: number; y: number };
}

type CellVisibility = "visible" | "explored";

type RenderCell = {
    cell: Cell;
    visibility: CellVisibility;
    hasPlayer: boolean;
    background: string;
};

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

function getTileBackground(
    cell: Cell,
    ownerGroupByUserId?: Readonly<Record<number, number>>,
    startOwners?: Record<string, number>,
): string {
    const resolveGroupId = (userId?: number | null) => {
        if (!userId) return null;
        return ownerGroupByUserId?.[userId] ?? null;
    };

    if (cell.tileCode === 80) {
        const key = `${cell.x}:${cell.y}`;
        const groupId = resolveGroupId(startOwners?.[key] ?? null);

        if (groupId === 1)
            return "linear-gradient(155deg, #2f68bf 0%, #183a72 100%)";
        if (groupId === 2)
            return "linear-gradient(155deg, #a93939 0%, #7f2727 100%)";
        if (groupId === 3)
            return "linear-gradient(155deg, #f2c94c 0%, #c58f16 100%)";
    }

    if (cell.structure_type === "base" && cell.structure_owner_user_id) {
        const groupId = resolveGroupId(cell.structure_owner_user_id);
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

function Map({
    grid,
    mapWidth,
    mapHeight,
    tileSize,
    gap,
    sightRange,
    playerPosition,
    onCellClick,
    startOwners = {},
    occupiedCellIndices,
    ownerGroupByUserId,
    explorationStorageKey,
    renderCenterPosition,
}: MapProps) {
    const fullWidth = mapWidth * tileSize + (mapWidth - 1) * gap;
    const fullHeight = mapHeight * tileSize + (mapHeight - 1) * gap;
    const step = tileSize + gap;

    // Track explored cells
    const exploredCellsRef = useRef<Set<number>>(new Set());
    const lastMapKeyRef = useRef<string>("");
    const lastExplorationStorageKeyRef = useRef<string>("");

    const cellsWithVisibility = useMemo<RenderCell[]>(() => {
        if (!Array.isArray(grid) || grid.length === 0) {
            return [];
        }

        const effectiveStorageKey = buildExplorationStorageKey(
            explorationStorageKey ?? "",
            mapWidth,
            mapHeight,
        );

        if (effectiveStorageKey !== lastExplorationStorageKeyRef.current) {
            lastExplorationStorageKeyRef.current = effectiveStorageKey;
            lastMapKeyRef.current = `${mapWidth}x${mapHeight}`;
            exploredCellsRef.current = effectiveStorageKey
                ? loadExploredCells(effectiveStorageKey)
                : new Set<number>();
        } else if (`${mapWidth}x${mapHeight}` !== lastMapKeyRef.current) {
            lastMapKeyRef.current = `${mapWidth}x${mapHeight}`;
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
                    hasPlayer: occupiedCellIndices?.has(key) ?? false,
                    background: getTileBackground(
                        cell,
                        ownerGroupByUserId,
                        startOwners,
                    ),
                });
            }
        }

        /**
         * IMPORTANT:
         * Do not render all explored cells of the entire map.
         * Render only a local window around the player.
         */
        const exploredBuffer = 3;
        const renderCenterX = renderCenterPosition?.x ?? playerPosition.x;
        const renderCenterY = renderCenterPosition?.y ?? playerPosition.y;

        const renderMinX = Math.max(
            0,
            Math.min(renderCenterX - sightRange - exploredBuffer, mapWidth - 1),
        );
        const renderMaxX = Math.min(
            mapWidth - 1,
            Math.max(renderCenterX + sightRange + exploredBuffer, 0),
        );
        const renderMinY = Math.max(
            0,
            Math.min(renderCenterY - sightRange - exploredBuffer, mapHeight - 1),
        );
        const renderMaxY = Math.min(
            mapHeight - 1,
            Math.max(renderCenterY + sightRange + exploredBuffer, 0),
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
                    hasPlayer: false,
                    background: getTileBackground(
                        cell,
                        ownerGroupByUserId,
                        startOwners,
                    ),
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
        explorationStorageKey,
        occupiedCellIndices,
        ownerGroupByUserId,
        startOwners,
        renderCenterPosition?.x,
        renderCenterPosition?.y,
    ]);

    useEffect(() => {
        const effectiveStorageKey = lastExplorationStorageKeyRef.current;
        if (!effectiveStorageKey) return;

        persistExploredCells(effectiveStorageKey, exploredCellsRef.current);
    }, [
        cellsWithVisibility,
        grid,
        mapWidth,
        mapHeight,
        playerPosition.x,
        playerPosition.y,
        sightRange,
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
            {cellsWithVisibility.map(
                ({ cell, visibility, hasPlayer, background }) => {
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
                            hasPlayer={hasPlayer}
                            background={background}
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
