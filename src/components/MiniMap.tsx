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

type CellVisibility = "visible" | "explored" | "unknown";

type MiniMapCellState = {
    cell: Cell;
    players: PlayerState[];
    visibility: CellVisibility;
    isViewportEdge: boolean;
    isPinged: boolean;
};

type MiniMapSize = "small" | "large";

const MAX_RENDERED_PLAYER_MARKERS = 3;
const SIZE_STORAGE_KEY = "minimap:size";
const LOCAL_PING_DURATION_MS = 1600;
const DEVICE_PIXEL_RATIO_FALLBACK = 1;

const PLAYER_MARKER_LAYOUT = [
    { x: 0.5, y: 0.5, radius: 0.18 },
    { x: 0.68, y: 0.28, radius: 0.14 },
    { x: 0.32, y: 0.68, radius: 0.14 },
];

const FALLBACK_UNKNOWN_CELL: Cell = {
    cell_id: 0,
    x: 0,
    y: 0,
    tileCode: 32,
    resource: null,
    barbel: null,
    monster: null,
    isPortal: false,
    isPlayer: false,
};

function MiniMap({
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
    const canvasRef = React.useRef<HTMLCanvasElement | null>(null);
    const canvasHostRef = React.useRef<HTMLDivElement | null>(null);
    const [canvasBounds, setCanvasBounds] = React.useState({
        width: 0,
        height: 0,
    });

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

    React.useEffect(() => {
        const host = canvasHostRef.current;
        if (!host || typeof ResizeObserver === "undefined") {
            return;
        }

        const updateBounds = () => {
            const rect = host.getBoundingClientRect();
            setCanvasBounds((prev) => {
                const nextWidth = Math.max(1, Math.round(rect.width));
                const nextHeight = Math.max(1, Math.round(rect.height));

                if (
                    prev.width === nextWidth &&
                    prev.height === nextHeight
                ) {
                    return prev;
                }

                return {
                    width: nextWidth,
                    height: nextHeight,
                };
            });
        };

        updateBounds();

        const observer = new ResizeObserver(() => {
            updateBounds();
        });

        observer.observe(host);

        return () => {
            observer.disconnect();
        };
    }, []);

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

    const playersByCellIndex = React.useMemo(() => {
        const next = new Map<number, PlayerState[]>();

        if (mapWidth <= 0 || mapHeight <= 0) {
            return next;
        }

        for (const player of players) {
            const idx = getCellIndex(
                player.position.x,
                player.position.y,
                mapWidth,
            );
            const existing = next.get(idx);

            if (existing) {
                existing.push(player);
            } else {
                next.set(idx, [player]);
            }
        }

        return next;
    }, [mapHeight, mapWidth, players]);

    const resolveCellState = React.useCallback(
        (x: number, y: number): MiniMapCellState => {
            const cellIndex = getCellIndex(x, y, mapWidth);
            const cell = grid[cellIndex] ?? {
                ...FALLBACK_UNKNOWN_CELL,
                cell_id: cellIndex + 1,
                x,
                y,
            };
            const isVisible = visibleCells.has(cellIndex);
            const isExplored = exploredCells.has(cellIndex);
            const visibility: CellVisibility = isVisible
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

            return {
                cell,
                players:
                    visibility === "visible"
                        ? (playersByCellIndex.get(cellIndex) ?? [])
                        : [],
                visibility,
                isViewportEdge,
                isPinged,
            };
        },
        [
            exploredCells,
            grid,
            localPingPoint,
            mapWidth,
            playersByCellIndex,
            viewportRect,
            visibleCells,
        ],
    );

    React.useEffect(() => {
        const canvas = canvasRef.current;
        if (
            !canvas ||
            !myPlayer ||
            mapWidth <= 0 ||
            mapHeight <= 0 ||
            canvasBounds.width <= 0 ||
            canvasBounds.height <= 0
        ) {
            return;
        }

        const dpr =
            typeof window !== "undefined"
                ? window.devicePixelRatio || DEVICE_PIXEL_RATIO_FALLBACK
                : DEVICE_PIXEL_RATIO_FALLBACK;

        canvas.width = Math.max(1, Math.round(canvasBounds.width * dpr));
        canvas.height = Math.max(1, Math.round(canvasBounds.height * dpr));
        canvas.style.width = `${canvasBounds.width}px`;
        canvas.style.height = `${canvasBounds.height}px`;

        const ctx = canvas.getContext("2d");
        if (!ctx) return;

        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        ctx.clearRect(0, 0, canvasBounds.width, canvasBounds.height);
        ctx.imageSmoothingEnabled = false;

        const cellWidth = canvasBounds.width / mapWidth;
        const cellHeight = canvasBounds.height / mapHeight;

        for (let y = 0; y < mapHeight; y++) {
            for (let x = 0; x < mapWidth; x++) {
                const state = resolveCellState(x, y);
                drawMiniMapCell(
                    ctx,
                    state,
                    x * cellWidth,
                    y * cellHeight,
                    cellWidth,
                    cellHeight,
                    myPlayerId,
                    activeUserId,
                );
            }
        }
    }, [
        activeUserId,
        canvasBounds.height,
        canvasBounds.width,
        mapHeight,
        mapWidth,
        myPlayer,
        myPlayerId,
        resolveCellState,
    ]);

    const resolveCanvasPoint = React.useCallback(
        (event: React.MouseEvent<HTMLCanvasElement>) => {
            const rect = event.currentTarget.getBoundingClientRect();
            if (rect.width <= 0 || rect.height <= 0) {
                return null;
            }

            const rawX = ((event.clientX - rect.left) / rect.width) * mapWidth;
            const rawY =
                ((event.clientY - rect.top) / rect.height) * mapHeight;

            return {
                x: clamp(Math.floor(rawX), 0, Math.max(0, mapWidth - 1)),
                y: clamp(Math.floor(rawY), 0, Math.max(0, mapHeight - 1)),
            };
        },
        [mapHeight, mapWidth],
    );

    const handleCanvasClick = React.useCallback(
        (event: React.MouseEvent<HTMLCanvasElement>) => {
            const point = resolveCanvasPoint(event);
            if (!point) return;

            const state = resolveCellState(point.x, point.y);
            if (state.visibility === "unknown") return;

            setLocalPingPoint(point);
            onCellPing?.(point);
        },
        [onCellPing, resolveCanvasPoint, resolveCellState],
    );

    const handleCanvasPointerMove = React.useCallback(
        (event: React.MouseEvent<HTMLCanvasElement>) => {
            const point = resolveCanvasPoint(event);
            if (!point) return;

            const state = resolveCellState(point.x, point.y);
            event.currentTarget.title = buildCellTitle(
                state.cell,
                state.players,
                state.visibility,
            );
        },
        [resolveCanvasPoint, resolveCellState],
    );

    const handleCanvasPointerLeave = React.useCallback(
        (event: React.MouseEvent<HTMLCanvasElement>) => {
            event.currentTarget.title = "";
        },
        [],
    );

    if (!mapWidth || !mapHeight || !myPlayer) {
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

            <div className={styles.grid}>
                <div
                    ref={canvasHostRef}
                    className={styles.canvasFrame}
                    style={{
                        aspectRatio: `${mapWidth} / ${mapHeight}`,
                    }}
                >
                    <canvas
                        ref={canvasRef}
                        className={styles.canvas}
                        onClick={handleCanvasClick}
                        onMouseMove={handleCanvasPointerMove}
                        onMouseLeave={handleCanvasPointerLeave}
                    />
                </div>
            </div>
        </aside>
    );
}

export default React.memo(MiniMap);

function clamp(value: number, min: number, max: number) {
    return Math.min(Math.max(value, min), max);
}

function drawMiniMapCell(
    ctx: CanvasRenderingContext2D,
    state: MiniMapCellState,
    left: number,
    top: number,
    width: number,
    height: number,
    myPlayerId?: number,
    activeUserId?: number,
) {
    const { cell, players, visibility, isViewportEdge, isPinged } = state;

    ctx.fillStyle = getTerrainFill(cell, visibility);
    ctx.fillRect(left, top, width, height);

    if (visibility === "explored") {
        ctx.fillStyle = "rgba(7, 12, 18, 0.55)";
        ctx.fillRect(left, top, width, height);
    }

    ctx.strokeStyle =
        visibility === "unknown"
            ? "rgba(15, 23, 32, 0.72)"
            : "rgba(5, 12, 20, 0.32)";
    ctx.lineWidth = 1;
    ctx.strokeRect(left + 0.5, top + 0.5, Math.max(0, width - 1), Math.max(0, height - 1));

    if (isViewportEdge) {
        ctx.strokeStyle = "rgba(186, 230, 253, 0.76)";
        ctx.lineWidth = Math.max(1, Math.min(width, height) * 0.08);
        ctx.strokeRect(
            left + ctx.lineWidth / 2,
            top + ctx.lineWidth / 2,
            Math.max(0, width - ctx.lineWidth),
            Math.max(0, height - ctx.lineWidth),
        );
    }

    if (visibility === "visible") {
        drawPointOfInterest(ctx, cell, left, top, width, height);

        const visiblePlayers = players.slice(0, MAX_RENDERED_PLAYER_MARKERS);
        visiblePlayers.forEach((player, index) => {
            const marker = PLAYER_MARKER_LAYOUT[index] ?? PLAYER_MARKER_LAYOUT[0];
            const centerX = left + width * marker.x;
            const centerY = top + height * marker.y;
            const radius = Math.max(1.5, Math.min(width, height) * marker.radius);

            ctx.beginPath();
            ctx.arc(centerX, centerY, radius, 0, Math.PI * 2);
            ctx.fillStyle = getGroupFill(player.group_id);
            ctx.fill();

            ctx.strokeStyle = "rgba(6, 11, 18, 0.85)";
            ctx.lineWidth = 1;
            ctx.stroke();

            if (player.user_id === myPlayerId) {
                ctx.beginPath();
                ctx.arc(centerX, centerY, radius + 1.5, 0, Math.PI * 2);
                ctx.strokeStyle = "rgba(34, 197, 94, 0.7)";
                ctx.lineWidth = 2;
                ctx.stroke();
            }

            if (player.user_id === activeUserId) {
                ctx.beginPath();
                ctx.arc(centerX, centerY, radius + 3.5, 0, Math.PI * 2);
                ctx.strokeStyle = "rgba(250, 204, 21, 0.92)";
                ctx.lineWidth = 1.5;
                ctx.stroke();
            }
        });
    }

    if (isPinged) {
        const centerX = left + width / 2;
        const centerY = top + height / 2;
        const radius = Math.max(3, Math.min(width, height) * 0.36);

        ctx.beginPath();
        ctx.arc(centerX, centerY, radius, 0, Math.PI * 2);
        ctx.strokeStyle = "rgba(186, 230, 253, 0.95)";
        ctx.lineWidth = Math.max(1, Math.min(width, height) * 0.08);
        ctx.stroke();

        ctx.beginPath();
        ctx.arc(centerX, centerY, radius * 0.58, 0, Math.PI * 2);
        ctx.strokeStyle = "rgba(56, 189, 248, 0.65)";
        ctx.lineWidth = Math.max(1, Math.min(width, height) * 0.06);
        ctx.stroke();
    }
}

function drawPointOfInterest(
    ctx: CanvasRenderingContext2D,
    cell: Cell,
    left: number,
    top: number,
    width: number,
    height: number,
) {
    const centerX = left + width / 2;
    const centerY = top + height / 2;
    const radius = Math.max(1.5, Math.min(width, height) * 0.14);

    if (cell.resource || cell.barbel) {
        ctx.beginPath();
        ctx.arc(centerX, centerY, radius, 0, Math.PI * 2);
        ctx.fillStyle = cell.barbel ? "#fdba74" : "#7dd3fc";
        ctx.fill();
        ctx.strokeStyle = cell.barbel
            ? "rgba(146, 64, 14, 0.9)"
            : "rgba(3, 105, 161, 0.85)";
        ctx.lineWidth = 1;
        ctx.stroke();
    }

    if (cell.monster) {
        ctx.beginPath();
        ctx.arc(centerX, centerY, radius + 0.5, 0, Math.PI * 2);
        ctx.fillStyle = "#fee2e2";
        ctx.fill();
        ctx.strokeStyle = "rgba(127, 39, 39, 0.8)";
        ctx.lineWidth = 1;
        ctx.stroke();
    }

    if (cell.structure_type) {
        const side = Math.max(3, Math.min(width, height) * 0.32);
        ctx.fillStyle = cell.is_under_construction
            ? "rgba(251, 191, 36, 0.9)"
            : "rgba(17, 24, 39, 0.78)";
        ctx.strokeStyle = cell.is_under_construction
            ? "rgba(120, 53, 15, 0.92)"
            : "rgba(255, 255, 255, 0.55)";
        ctx.lineWidth = 1;
        ctx.fillRect(centerX - side / 2, centerY - side / 2, side, side);
        ctx.strokeRect(centerX - side / 2, centerY - side / 2, side, side);
    }

    if (cell.isPortal) {
        const portalRadius = Math.max(3, Math.min(width, height) * 0.22);
        ctx.beginPath();
        ctx.arc(centerX, centerY, portalRadius, 0, Math.PI * 2);
        ctx.strokeStyle = "rgba(240, 253, 250, 0.92)";
        ctx.lineWidth = 1;
        ctx.stroke();
        ctx.beginPath();
        ctx.arc(centerX, centerY, portalRadius + 1.5, 0, Math.PI * 2);
        ctx.strokeStyle = "rgba(45, 212, 191, 0.75)";
        ctx.lineWidth = 1;
        ctx.stroke();
    }
}

function getTerrainFill(cell: Cell, visibility: CellVisibility) {
    if (visibility === "unknown") {
        return "#0b111a";
    }

    if (visibility === "visible") {
        if (cell.isPortal) return "#158b86";
        if (cell.structure_type) return "#7a63d1";
        if (cell.monster) return "#8d3434";
        if (cell.resource) return "#2f8a71";
        if (cell.barbel) return "#a26d36";
    }

    if (cell.tileCode === 32) return "#32404b";
    if (cell.tileCode === 80) return "#2563eb";
    if (cell.tileCode === 48) return "#7d8995";
    return "#4b2f28";
}

function getGroupFill(groupId?: number) {
    switch (groupId) {
        case 1:
            return "#60a5fa";
        case 2:
            return "#f87171";
        case 3:
            return "#facc15";
        default:
            return "#f8fafc";
    }
}

function buildCellTitle(
    cell: Cell,
    players: PlayerState[],
    visibility: CellVisibility,
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
