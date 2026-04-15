"use client";

import React from "react";
import type { Cell, PlayerState } from "@/types";
import styles from "@/styles/MiniMap.module.css";

type MiniMapProps = {
    grid: Cell[];
    mapWidth: number;
    mapHeight: number;
    players: PlayerState[];
    myPlayerId?: number;
    activeUserId?: number;
    viewportCells?: {
        width: number;
        height: number;
    };
};

type OrderedCell = {
    cell: Cell;
    players: PlayerState[];
    isViewportEdge: boolean;
};

const MAX_RENDERED_PLAYER_MARKERS = 3;

export default function MiniMap({
    grid,
    mapWidth,
    mapHeight,
    players,
    myPlayerId,
    activeUserId,
    viewportCells,
}: MiniMapProps) {
    const myPlayer = React.useMemo(
        () => players.find((player) => player.user_id === myPlayerId),
        [players, myPlayerId],
    );

    const viewportRect = React.useMemo(() => {
        if (
            !myPlayer ||
            !viewportCells ||
            mapWidth <= 0 ||
            mapHeight <= 0 ||
            viewportCells.width <= 0 ||
            viewportCells.height <= 0
        ) {
            return null;
        }

        const width = Math.min(mapWidth, viewportCells.width);
        const height = Math.min(mapHeight, viewportCells.height);
        const halfWidth = Math.floor(width / 2);
        const halfHeight = Math.floor(height / 2);

        const minX = clamp(myPlayer.position.x - halfWidth, 0, mapWidth - width);
        const minY = clamp(
            myPlayer.position.y - halfHeight,
            0,
            mapHeight - height,
        );

        return {
            minX,
            minY,
            maxX: minX + width - 1,
            maxY: minY + height - 1,
        };
    }, [myPlayer, viewportCells, mapWidth, mapHeight]);

    const orderedCells = React.useMemo<OrderedCell[]>(() => {
        if (mapWidth <= 0 || mapHeight <= 0) return [];

        const cellMap = new globalThis.Map<string, Cell>();
        for (const cell of grid) {
            cellMap.set(`${cell.x}:${cell.y}`, cell);
        }

        const playersByCell = new globalThis.Map<string, PlayerState[]>();
        for (const player of players) {
            const key = `${player.position.x}:${player.position.y}`;
            const existing = playersByCell.get(key);
            if (existing) {
                existing.push(player);
            } else {
                playersByCell.set(key, [player]);
            }
        }

        const result: OrderedCell[] = [];
        for (let y = 0; y < mapHeight; y++) {
            for (let x = 0; x < mapWidth; x++) {
                const key = `${x}:${y}`;
                const cell =
                    cellMap.get(key) ??
                    ({
                        cell_id: y * mapWidth + x + 1,
                        x,
                        y,
                        tileCode: 32,
                        resource: null,
                        barbel: null,
                        monster: null,
                        isPortal: false,
                        isPlayer: false,
                    } satisfies Cell);

                const isViewportEdge =
                    !!viewportRect &&
                    x >= viewportRect.minX &&
                    x <= viewportRect.maxX &&
                    y >= viewportRect.minY &&
                    y <= viewportRect.maxY &&
                    (x === viewportRect.minX ||
                        x === viewportRect.maxX ||
                        y === viewportRect.minY ||
                        y === viewportRect.maxY);

                result.push({
                    cell,
                    players: playersByCell.get(key) ?? [],
                    isViewportEdge,
                });
            }
        }

        return result;
    }, [grid, players, mapWidth, mapHeight, viewportRect]);

    const counts = React.useMemo(() => {
        let monsters = 0;
        let resources = 0;
        let barrels = 0;
        let structures = 0;

        for (const cell of grid) {
            if (cell.monster) monsters++;
            if (cell.resource) resources++;
            if (cell.barbel) barrels++;
            if (cell.structure_type) structures++;
        }

        return { monsters, resources, barrels, structures };
    }, [grid]);

    if (!mapWidth || !mapHeight || orderedCells.length === 0) {
        return null;
    }

    return (
        <aside className={styles.panel} aria-label="Battlefield minimap">
            <div className={styles.header}>
                <div>
                    <div className={styles.title}>Battlefield</div>
                    <div className={styles.subtitle}>
                        {mapWidth}x{mapHeight} · {players.length} players ·{" "}
                        {counts.monsters} monsters
                    </div>
                </div>
            </div>

            <div className={styles.legend}>
                <span className={styles.legendItem}>
                    <span
                        className={`${styles.legendSwatch} ${styles.youSwatch}`}
                    />
                    You
                </span>
                <span className={styles.legendItem}>
                    <span
                        className={`${styles.legendSwatch} ${styles.activeSwatch}`}
                    />
                    Turn
                </span>
                <span className={styles.legendItem}>
                    <span
                        className={`${styles.legendSwatch} ${styles.monsterSwatch}`}
                    />
                    Monsters
                </span>
                <span className={styles.legendItem}>
                    <span
                        className={`${styles.legendSwatch} ${styles.resourceSwatch}`}
                    />
                    Loot
                </span>
            </div>

            <div
                className={styles.grid}
                style={{
                    gridTemplateColumns: `repeat(${mapWidth}, minmax(0, 1fr))`,
                    aspectRatio: `${mapWidth} / ${mapHeight}`,
                }}
            >
                {orderedCells.map(({ cell, players: cellPlayers, isViewportEdge }) => {
                    const playerMarkers = cellPlayers.slice(
                        0,
                        MAX_RENDERED_PLAYER_MARKERS,
                    );

                    return (
                        <div
                            key={`${cell.x}:${cell.y}`}
                            className={getCellClassName(cell, isViewportEdge)}
                            title={buildCellTitle(cell, cellPlayers)}
                        >
                            {cell.resource || cell.barbel ? (
                                <span
                                    className={`${styles.poiMarker} ${
                                        cell.barbel
                                            ? styles.barrelMarker
                                            : styles.resourceMarker
                                    }`}
                                />
                            ) : null}

                            {cell.monster ? (
                                <span
                                    className={`${styles.poiMarker} ${styles.monsterMarker}`}
                                />
                            ) : null}

                            {cell.structure_type ? (
                                <span
                                    className={`${styles.poiMarker} ${
                                        cell.is_under_construction
                                            ? styles.constructionMarker
                                            : styles.structureMarker
                                    }`}
                                />
                            ) : null}

                            {cell.isPortal ? (
                                <span
                                    className={`${styles.poiMarker} ${styles.portalMarker}`}
                                />
                            ) : null}

                            {playerMarkers.map((player, index) => (
                                <span
                                    key={player.user_id}
                                    className={getPlayerMarkerClassName(
                                        player,
                                        index,
                                        myPlayerId,
                                        activeUserId,
                                    )}
                                    style={getPlayerMarkerStyle(index)}
                                />
                            ))}
                        </div>
                    );
                })}
            </div>

            <div className={styles.footer}>
                <span>{counts.resources} resources</span>
                <span>{counts.barrels} barrels</span>
                <span>{counts.structures} structures</span>
            </div>
        </aside>
    );
}

function clamp(value: number, min: number, max: number) {
    return Math.min(Math.max(value, min), max);
}

function getCellClassName(cell: Cell, isViewportEdge: boolean) {
    return [
        styles.cell,
        getTerrainClassName(cell),
        isViewportEdge ? styles.viewportEdge : "",
    ]
        .filter(Boolean)
        .join(" ");
}

function getTerrainClassName(cell: Cell) {
    if (cell.isPortal) return styles.portalCell;
    if (cell.tileCode === 32) return styles.blockedCell;
    if (cell.tileCode === 80) return styles.startCell;
    if (cell.structure_type) return styles.structureCell;
    if (cell.monster) return styles.monsterCell;
    if (cell.resource) return styles.resourceCell;
    if (cell.barbel) return styles.barrelCell;
    if (cell.tileCode === 48) return styles.walkableCell;
    return styles.borderCell;
}

function getPlayerMarkerClassName(
    player: PlayerState,
    index: number,
    myPlayerId?: number,
    activeUserId?: number,
) {
    return [
        styles.playerMarker,
        getGroupClassName(player.group_id),
        player.user_id === myPlayerId ? styles.myPlayerMarker : "",
        player.user_id === activeUserId ? styles.activePlayerMarker : "",
        index > 0 ? styles.stackMarker : "",
    ]
        .filter(Boolean)
        .join(" ");
}

function getGroupClassName(groupId?: number) {
    switch (groupId) {
        case 1:
            return styles.groupOne;
        case 2:
            return styles.groupTwo;
        case 3:
            return styles.groupThree;
        default:
            return styles.groupNeutral;
    }
}

function getPlayerMarkerStyle(index: number): React.CSSProperties {
    const offsets = [
        { top: "50%", left: "50%" },
        { top: "28%", left: "68%" },
        { top: "68%", left: "32%" },
    ];

    const offset = offsets[index] ?? offsets[0];

    return {
        top: offset.top,
        left: offset.left,
    };
}

function buildCellTitle(cell: Cell, players: PlayerState[]) {
    const parts = [`${cell.x}:${cell.y}`];

    if (players.length > 0) {
        parts.push(
            `players: ${players.map((player) => player.name).join(", ")}`,
        );
    }
    if (cell.monster) {
        parts.push(`monster: ${cell.monster.name}`);
    }
    if (cell.resource) {
        parts.push(`resource: ${cell.resource.type}`);
    }
    if (cell.barbel) {
        parts.push("barrel");
    }
    if (cell.isPortal) {
        parts.push("portal");
    }
    if (cell.structure_type) {
        parts.push(
            cell.is_under_construction
                ? `${cell.structure_type} (building)`
                : cell.structure_type,
        );
    }

    return parts.join(" · ");
}
