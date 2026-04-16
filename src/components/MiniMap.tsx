"use client";

import React from "react";
import type { Cell, PlayerState } from "@/types";
import {
    buildExplorationStorageKey,
    getCellIndex,
    loadExploredCells,
    persistExploredCells,
} from "@/utils/fogOfWar";
import styles from "@/styles/MiniMap.module.css";

type MiniMapProps = {
    grid: Cell[];
    mapWidth: number;
    mapHeight: number;
    players: PlayerState[];
    instanceId: string;
    myPlayerId?: number;
    activeUserId?: number;
    sightRange: number;
    viewportCells?: {
        width: number;
        height: number;
    };
    cameraCenterPosition?: { x: number; y: number } | null;
    onCellPing?: (point: { x: number; y: number }) => void;
};

type OrderedCell = {
    cell: Cell;
    players: PlayerState[];
    visibility: "visible" | "explored" | "unknown";
    isViewportEdge: boolean;
    isPinged: boolean;
};

type MiniMapSize = "small" | "large";

const MAX_RENDERED_PLAYER_MARKERS = 3;
const SIZE_STORAGE_KEY = "minimap:size";
const LOCAL_PING_DURATION_MS = 1600;

export default function MiniMap({
    grid,
    mapWidth,
    mapHeight,
    players,
    instanceId,
    myPlayerId,
    activeUserId,
    sightRange,
    viewportCells,
    cameraCenterPosition,
    onCellPing,
}: MiniMapProps) {
    const [size, setSize] = React.useState<MiniMapSize>("small");
    const [localPingPoint, setLocalPingPoint] = React.useState<{
        x: number;
        y: number;
    } | null>(null);

    const myPlayer = React.useMemo(
        () => players.find((player) => player.user_id === myPlayerId),
        [players, myPlayerId],
    );

    React.useEffect(() => {
        if (typeof window === "undefined") return;

        try {
            const savedSize = window.sessionStorage.getItem(SIZE_STORAGE_KEY);
            if (savedSize === "small" || savedSize === "large") {
                setSize(savedSize);
            }
        } catch {
            // ignore storage failures
        }
    }, []);

    React.useEffect(() => {
        if (typeof window === "undefined") return;

        try {
            window.sessionStorage.setItem(SIZE_STORAGE_KEY, size);
        } catch {
            // ignore storage failures
        }
    }, [size]);

    React.useEffect(() => {
        if (!localPingPoint) return;

        const timeoutId = window.setTimeout(() => {
            setLocalPingPoint(null);
        }, LOCAL_PING_DURATION_MS);

        return () => {
            window.clearTimeout(timeoutId);
        };
    }, [localPingPoint]);

    React.useEffect(() => {
        setLocalPingPoint(null);
    }, [instanceId, myPlayerId]);

    const explorationStorageKey = React.useMemo(() => {
        if (!instanceId || !myPlayerId) return "";

        return buildExplorationStorageKey(
            `fog-explored:${instanceId}:${myPlayerId}`,
            mapWidth,
            mapHeight,
        );
    }, [instanceId, mapHeight, mapWidth, myPlayerId]);

    const visibleCells = React.useMemo(() => {
        const next = new Set<number>();

        if (!myPlayer || mapWidth <= 0 || mapHeight <= 0) {
            return next;
        }

        const minX = Math.max(0, myPlayer.position.x - sightRange);
        const maxX = Math.min(mapWidth - 1, myPlayer.position.x + sightRange);
        const minY = Math.max(0, myPlayer.position.y - sightRange);
        const maxY = Math.min(mapHeight - 1, myPlayer.position.y + sightRange);

        for (let y = minY; y <= maxY; y++) {
            for (let x = minX; x <= maxX; x++) {
                next.add(getCellIndex(x, y, mapWidth));
            }
        }

        return next;
    }, [mapHeight, mapWidth, myPlayer, sightRange]);

    const exploredCells = React.useMemo(() => {
        const next = loadExploredCells(explorationStorageKey);

        visibleCells.forEach((cellIndex) => {
            next.add(cellIndex);
        });

        return next;
    }, [explorationStorageKey, visibleCells]);

    React.useEffect(() => {
        if (!explorationStorageKey) return;
        persistExploredCells(explorationStorageKey, exploredCells);
    }, [explorationStorageKey, exploredCells]);

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

        const center = cameraCenterPosition ?? myPlayer.position;
        const width = Math.min(mapWidth, viewportCells.width);
        const height = Math.min(mapHeight, viewportCells.height);
        const halfWidth = Math.floor(width / 2);
        const halfHeight = Math.floor(height / 2);

        const minX = clamp(center.x - halfWidth, 0, mapWidth - width);
        const minY = clamp(center.y - halfHeight, 0, mapHeight - height);

        return {
            minX,
            minY,
            maxX: minX + width - 1,
            maxY: minY + height - 1,
        };
    }, [
        cameraCenterPosition,
        mapHeight,
        mapWidth,
        myPlayer,
        viewportCells,
    ]);

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
                const fallbackCell: Cell = {
                    cell_id: y * mapWidth + x + 1,
                    x,
                    y,
                    tileCode: 32,
                    resource: null,
                    barbel: null,
                    monster: null,
                    isPortal: false,
                    isPlayer: false,
                };
                const cell = cellMap.get(key) ?? fallbackCell;
                const cellIndex = getCellIndex(x, y, mapWidth);
                const isVisible = visibleCells.has(cellIndex);
                const isExplored = exploredCells.has(cellIndex);
                const visibility = isVisible
                    ? "visible"
                    : isExplored
                      ? "explored"
                      : "unknown";
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
                const isPinged =
                    !!localPingPoint &&
                    localPingPoint.x === x &&
                    localPingPoint.y === y;

                result.push({
                    cell,
                    players:
                        visibility === "visible"
                            ? (playersByCell.get(key) ?? [])
                            : [],
                    visibility,
                    isViewportEdge,
                    isPinged,
                });
            }
        }

        return result;
    }, [
        exploredCells,
        grid,
        localPingPoint,
        mapHeight,
        mapWidth,
        players,
        viewportRect,
        visibleCells,
    ]);

    const handleCellClick = React.useCallback(
        (cell: Cell, visibility: OrderedCell["visibility"]) => {
            if (visibility === "unknown") return;

            const point = { x: cell.x, y: cell.y };
            setLocalPingPoint(point);
            onCellPing?.(point);
        },
        [onCellPing],
    );

    if (!mapWidth || !mapHeight || orderedCells.length === 0 || !myPlayer) {
        return null;
    }

    return (
        <aside
            className={`${styles.panel} ${
                size === "large" ? styles.panelLarge : styles.panelSmall
            }`}
            aria-label="Battlefield minimap"
        >
            <div className={styles.header}>
                <div>
                    <div className={styles.title}>Minimap</div>
                    <div className={styles.subtitle}>
                        {mapWidth}x{mapHeight} · vision {sightRange}
                    </div>
                </div>

                <div className={styles.controls}>
                    <button
                        type="button"
                        className={`${styles.sizeButton} ${
                            size === "small" ? styles.sizeButtonActive : ""
                        }`}
                        onClick={() => setSize("small")}
                        aria-pressed={size === "small"}
                    >
                        Small
                    </button>
                    <button
                        type="button"
                        className={`${styles.sizeButton} ${
                            size === "large" ? styles.sizeButtonActive : ""
                        }`}
                        onClick={() => setSize("large")}
                        aria-pressed={size === "large"}
                    >
                        Large
                    </button>
                </div>
            </div>

            <div className={styles.hint}>Click a known cell to focus and ping.</div>

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
                        className={`${styles.legendSwatch} ${styles.unknownSwatch}`}
                    />
                    Fog
                </span>
            </div>

            <div
                className={styles.grid}
                style={{
                    gridTemplateColumns: `repeat(${mapWidth}, minmax(0, 1fr))`,
                    aspectRatio: `${mapWidth} / ${mapHeight}`,
                }}
            >
                {orderedCells.map(
                    ({ cell, players: cellPlayers, visibility, isViewportEdge, isPinged }) => {
                        const playerMarkers = cellPlayers.slice(
                            0,
                            MAX_RENDERED_PLAYER_MARKERS,
                        );

                        return (
                            <button
                                key={`${cell.x}:${cell.y}`}
                                type="button"
                                className={getCellClassName(
                                    cell,
                                    visibility,
                                    isViewportEdge,
                                )}
                                title={buildCellTitle(
                                    cell,
                                    cellPlayers,
                                    visibility,
                                )}
                                onClick={() => handleCellClick(cell, visibility)}
                                disabled={visibility === "unknown"}
                            >
                                {visibility === "visible" &&
                                (cell.resource || cell.barbel) ? (
                                    <span
                                        className={`${styles.poiMarker} ${
                                            cell.barbel
                                                ? styles.barrelMarker
                                                : styles.resourceMarker
                                        }`}
                                    />
                                ) : null}

                                {visibility === "visible" && cell.monster ? (
                                    <span
                                        className={`${styles.poiMarker} ${styles.monsterMarker}`}
                                    />
                                ) : null}

                                {visibility === "visible" &&
                                cell.structure_type ? (
                                    <span
                                        className={`${styles.poiMarker} ${
                                            cell.is_under_construction
                                                ? styles.constructionMarker
                                                : styles.structureMarker
                                        }`}
                                    />
                                ) : null}

                                {visibility === "visible" && cell.isPortal ? (
                                    <span
                                        className={`${styles.poiMarker} ${styles.portalMarker}`}
                                    />
                                ) : null}

                                {visibility === "visible"
                                    ? playerMarkers.map((player, index) => (
                                          <span
                                              key={player.user_id}
                                              className={getPlayerMarkerClassName(
                                                  player,
                                                  index,
                                                  myPlayerId,
                                                  activeUserId,
                                              )}
                                              style={getPlayerMarkerStyle(
                                                  index,
                                              )}
                                          />
                                      ))
                                    : null}

                                {isPinged ? (
                                    <span className={styles.pingMarker} />
                                ) : null}
                            </button>
                        );
                    },
                )}
            </div>
        </aside>
    );
}

function clamp(value: number, min: number, max: number) {
    return Math.min(Math.max(value, min), max);
}

function getCellClassName(
    cell: Cell,
    visibility: OrderedCell["visibility"],
    isViewportEdge: boolean,
) {
    return [
        styles.cellButton,
        styles.cell,
        getTerrainClassName(cell, visibility),
        visibility === "visible"
            ? styles.visibleCell
            : visibility === "explored"
              ? styles.exploredCell
              : styles.unknownCell,
        isViewportEdge ? styles.viewportEdge : "",
    ]
        .filter(Boolean)
        .join(" ");
}

function getTerrainClassName(
    cell: Cell,
    visibility: OrderedCell["visibility"],
) {
    if (visibility === "unknown") return styles.unknownTerrain;

    if (visibility === "visible") {
        if (cell.isPortal) return styles.portalCell;
        if (cell.structure_type) return styles.structureCell;
        if (cell.monster) return styles.monsterCell;
        if (cell.resource) return styles.resourceCell;
        if (cell.barbel) return styles.barrelCell;
    }

    if (cell.tileCode === 32) return styles.blockedCell;
    if (cell.tileCode === 80) return styles.startCell;
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

function buildCellTitle(
    cell: Cell,
    players: PlayerState[],
    visibility: OrderedCell["visibility"],
) {
    const parts = [`${cell.x}:${cell.y}`];

    if (visibility === "unknown") {
        parts.push("unexplored");
        return parts.join(" · ");
    }

    parts.push(visibility === "visible" ? "visible" : "explored");

    if (visibility !== "visible") {
        return parts.join(" · ");
    }

    if (players.length > 0) {
        parts.push(`players: ${players.map((player) => player.name).join(", ")}`);
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
