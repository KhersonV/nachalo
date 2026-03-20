//==================================
// src/components/Map.tsx
//==================================

"use client";

import React, { useMemo, useRef } from "react";
import { Cell, PlayerState } from "@/types/GameTypes";
import { buildPlayerMap } from "@/utils/buildPlayerMap";
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
    // list of players (passed from parent to avoid internal selectors)
    players?: PlayerState[];
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

    // ✅ Мемоизированный playerMap — теперь зависит только от переданных players
    const playerMap = useMemo(() => buildPlayerMap(players), [players]);

    // Fog-of-war: запоминаем исследованные клетки (не сбрасываются при ходе)
    const exploredCellsRef = useRef<Set<string>>(new Set());
    const lastMapKeyRef = useRef<string>("");

    // ✅ Мемоизированные клетки: только видимая область + explored set
    const cellsWithVisibility = useMemo(() => {
        if (!Array.isArray(grid) || grid.length === 0) return [];

        // Сброс при смене карты (новый матч / другие размеры)
        const mapKey = `${mapWidth}x${mapHeight}`;
        if (mapKey !== lastMapKeyRef.current) {
            lastMapKeyRef.current = mapKey;
            exploredCellsRef.current = new Set<string>();
        }

        // Индекс для быстрого доступа по координатам
        // Используем глобальный Map через globalThis, чтобы не конфликтовать
        // с именем компонента `Map` в этом модуле.
        const index = new globalThis.Map<string, Cell>();
        for (const c of grid) {
            index.set(`${c.x}-${c.y}`, c);
        }

        const result: {
            cell: Cell;
            visibility: "visible" | "explored";
            player: PlayerState | null;
        }[] = [];

        const seenKeys = new Set<string>();

        // Проходим только по области видимости
        const minX = Math.max(0, playerPosition.x - sightRange);
        const maxX = Math.min(mapWidth - 1, playerPosition.x + sightRange);
        const minY = Math.max(0, playerPosition.y - sightRange);
        const maxY = Math.min(mapHeight - 1, playerPosition.y + sightRange);

        for (let x = minX; x <= maxX; x++) {
            for (let y = minY; y <= maxY; y++) {
                const key = `${x}-${y}`;
                const cell = index.get(key);
                if (!cell) continue;
                exploredCellsRef.current.add(key);
                seenKeys.add(key);
                result.push({
                    cell,
                    visibility: "visible",
                    player: playerMap.get(key) || null,
                });
            }
        }

        // Добавляем explored клетки, которых нет в видимой области
        for (const key of Array.from(exploredCellsRef.current)) {
            if (seenKeys.has(key)) continue;
            const cell = index.get(key);
            if (!cell) continue;
            result.push({ cell, visibility: "explored", player: null });
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
            {cellsWithVisibility.map(({ cell, visibility, player }) => (
                <div
                    key={`${cell.x}-${cell.y}`}
                    style={{
                        position: "absolute",
                        left: cell.x * (tileSize + gap),
                        top: cell.y * (tileSize + gap),
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
            ))}
        </div>
    );
}

export default React.memo(Map);
