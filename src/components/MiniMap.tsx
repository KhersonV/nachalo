"use client";

import React from "react";
import type { Cell, PlayerState } from "@/types";
import {
  buildExplorationStorageKey,
  buildTeamExplorationBaseKey,
  collectVisibleCellIndices,
  getCellIndex,
  isSameTeamPlayer,
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
  onCollapse?: () => void;
  compact?: boolean;
  panelClassName?: string;
};

type CellVisibility = "visible" | "explored" | "unknown";

type MiniMapCellState = {
  cell: Cell;
  players: PlayerState[];
  visibility: CellVisibility;
  isViewportEdge: boolean;
  isPinged: boolean;
};

type MiniMapCellButtonProps = {
  state: MiniMapCellState;
  myPlayerId?: number;
  activeUserId?: number;
  onCellClick: (state: MiniMapCellState) => void;
};

type MiniMapSize = "small" | "large";

const MAX_RENDERED_PLAYER_MARKERS = 3;
const SIZE_STORAGE_KEY = "minimap:size";
const LOCAL_PING_DURATION_MS = 1600;

const PLAYER_MARKER_LAYOUT = [
  { x: 0.5, y: 0.5 },
  { x: 0.68, y: 0.28 },
  { x: 0.32, y: 0.68 },
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

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
}

function areViewportCellsEqual(
  prev?: { width: number; height: number },
  next?: { width: number; height: number },
) {
  return (
    (prev?.width ?? 0) === (next?.width ?? 0) &&
    (prev?.height ?? 0) === (next?.height ?? 0)
  );
}

function arePointsEqual(
  prev: { x: number; y: number } | null,
  next: { x: number; y: number } | null,
) {
  return (
    (prev?.x ?? -1) === (next?.x ?? -1) &&
    (prev?.y ?? -1) === (next?.y ?? -1)
  );
}

function getCellClassName(state: MiniMapCellState) {
  const classNames = [styles.cellButton, styles.cell];

  if (state.visibility === "visible") {
    classNames.push(styles.visibleCell);
  } else if (state.visibility === "explored") {
    classNames.push(styles.exploredCell);
  } else {
    classNames.push(styles.unknownCell);
  }

  if (state.visibility !== "unknown") {
    classNames.push(getTerrainClassName(state.cell));
  }

  if (state.isViewportEdge) {
    classNames.push(styles.viewportEdge);
  }

  return classNames.join(" ");
}

function getTerrainClassName(cell: Cell) {
  if (cell.isPortal) return styles.portalCell;
  if (cell.structure_type) return styles.structureCell;
  if (cell.monster) return styles.monsterCell;
  if (cell.resource) return styles.resourceCell;
  if (cell.barbel) return styles.barrelCell;
  if (cell.tileCode === 32) return styles.blockedCell;
  if (cell.tileCode === 80) return styles.startCell;
  if (cell.tileCode === 48) return styles.walkableCell;
  return styles.borderCell;
}

function renderPointOfInterest(cell: Cell) {
  if (cell.is_under_construction) {
    return (
      <span className={`${styles.poiMarker} ${styles.constructionMarker}`} />
    );
  }

  if (cell.structure_type) {
    return <span className={`${styles.poiMarker} ${styles.structureMarker}`} />;
  }

  if (cell.isPortal) {
    return <span className={`${styles.poiMarker} ${styles.portalMarker}`} />;
  }

  if (cell.monster) {
    return <span className={`${styles.poiMarker} ${styles.monsterMarker}`} />;
  }

  if (cell.barbel) {
    return <span className={`${styles.poiMarker} ${styles.barrelMarker}`} />;
  }

  if (cell.resource) {
    return <span className={`${styles.poiMarker} ${styles.resourceMarker}`} />;
  }

  return null;
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

function getPlayerMarkerClassName(
  player: PlayerState,
  myPlayerId?: number,
  activeUserId?: number,
  isStacked = false,
) {
  const classNames = [styles.playerMarker, getGroupClassName(player.group_id)];

  if (isStacked) classNames.push(styles.stackMarker);
  if (player.user_id === myPlayerId) classNames.push(styles.myPlayerMarker);
  if (player.user_id === activeUserId)
    classNames.push(styles.activePlayerMarker);

  return classNames.join(" ");
}

function areRenderableCellsEqual(prev: Cell, next: Cell) {
  return (
    prev === next ||
    (prev.cell_id === next.cell_id &&
      prev.x === next.x &&
      prev.y === next.y &&
      prev.tileCode === next.tileCode &&
      prev.isPortal === next.isPortal &&
      !!prev.barbel === !!next.barbel &&
      (prev.resource?.type ?? "") === (next.resource?.type ?? "") &&
      (prev.monster?.name ?? "") === (next.monster?.name ?? "") &&
      (prev.monster?.db_instance_id ?? prev.monster?.id ?? 0) ===
        (next.monster?.db_instance_id ?? next.monster?.id ?? 0) &&
      (prev.structure_type ?? "") === (next.structure_type ?? "") &&
      !!prev.is_under_construction === !!next.is_under_construction)
  );
}

function areRenderablePlayersEqual(prev: PlayerState[], next: PlayerState[]) {
  if (prev === next) return true;
  if (prev.length !== next.length) return false;

  for (let i = 0; i < prev.length; i++) {
    if (
      prev[i]?.user_id !== next[i]?.user_id ||
      prev[i]?.group_id !== next[i]?.group_id ||
      prev[i]?.name !== next[i]?.name
    ) {
      return false;
    }
  }

  return true;
}

function areMiniMapCellStatesEqual(
  prev: MiniMapCellState,
  next: MiniMapCellState,
) {
  return (
    prev.visibility === next.visibility &&
    prev.isViewportEdge === next.isViewportEdge &&
    prev.isPinged === next.isPinged &&
    areRenderableCellsEqual(prev.cell, next.cell) &&
    areRenderablePlayersEqual(prev.players, next.players)
  );
}

function areMiniMapCellButtonPropsEqual(
  prev: MiniMapCellButtonProps,
  next: MiniMapCellButtonProps,
) {
  return (
    prev.myPlayerId === next.myPlayerId &&
    prev.activeUserId === next.activeUserId &&
    prev.onCellClick === next.onCellClick &&
    areMiniMapCellStatesEqual(prev.state, next.state)
  );
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
  if (cell.monster) parts.push(`monster: ${cell.monster.name}`);
  if (cell.resource) parts.push(`resource: ${cell.resource.type}`);
  if (cell.barbel) parts.push("barrel");
  if (cell.isPortal) parts.push("portal");
  if (cell.structure_type) {
    parts.push(
      cell.is_under_construction
        ? `${cell.structure_type} (building)`
        : cell.structure_type,
    );
  }

  return parts.join(" · ");
}

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
  onCollapse,
  compact = false,
  panelClassName = "",
}: MiniMapProps) {
  const [size, setSize] = React.useState<MiniMapSize>("small");
  const [localPingPoint, setLocalPingPoint] = React.useState<{
    x: number;
    y: number;
  } | null>(null);
  const lastPersistedExplorationRef = React.useRef("");

  const deferredGrid = React.useDeferredValue(grid);
  const deferredPlayers = React.useDeferredValue(players);
  const deferredViewportCells = React.useDeferredValue(viewportCells);
  const deferredCameraCenterPosition =
    React.useDeferredValue(cameraCenterPosition);

  const myPlayer = React.useMemo(
    () => deferredPlayers.find((player) => player.user_id === myPlayerId),
    [deferredPlayers, myPlayerId],
  );

  const teamPlayers = React.useMemo(
    () =>
      deferredPlayers.filter((player) =>
        isSameTeamPlayer(player, myPlayerId, myPlayer?.group_id),
      ),
    [deferredPlayers, myPlayer?.group_id, myPlayerId],
  );

  const effectiveSightRange = React.useMemo(
    () =>
      teamPlayers.reduce(
        (maxRange, player) =>
          Math.max(maxRange, player.sightRange ?? sightRange),
        myPlayer?.sightRange ?? sightRange,
      ),
    [myPlayer?.sightRange, sightRange, teamPlayers],
  );

  React.useEffect(() => {
    if (typeof window === "undefined") return;

    try {
      const savedSize = window.sessionStorage.getItem(SIZE_STORAGE_KEY);
      if (savedSize === "small" || savedSize === "large") {
        setSize(savedSize);
      }
    } catch {}
  }, []);

  React.useEffect(() => {
    if (typeof window === "undefined") return;

    try {
      window.sessionStorage.setItem(SIZE_STORAGE_KEY, size);
    } catch {}
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
    return buildExplorationStorageKey(
      buildTeamExplorationBaseKey(instanceId, myPlayerId, myPlayer?.group_id),
      mapWidth,
      mapHeight,
    );
  }, [instanceId, mapHeight, mapWidth, myPlayer?.group_id, myPlayerId]);

  const visibleCells = React.useMemo(
    () =>
      collectVisibleCellIndices(
        teamPlayers.map((player) => ({
          position: player.position,
          sightRange: player.sightRange ?? effectiveSightRange,
        })),
        mapWidth,
        mapHeight,
      ),
    [effectiveSightRange, mapHeight, mapWidth, teamPlayers],
  );

  const exploredCells = React.useMemo(() => {
    const next = loadExploredCells(explorationStorageKey);

    visibleCells.forEach((cellIndex) => {
      next.add(cellIndex);
    });

    return next;
  }, [explorationStorageKey, visibleCells]);

  React.useEffect(() => {
    if (!explorationStorageKey) {
      lastPersistedExplorationRef.current = "";
      return;
    }

    const encoded = JSON.stringify(
      Array.from(exploredCells.values()).sort((a, b) => a - b),
    );
    const persistKey = `${explorationStorageKey}:${encoded}`;

    if (lastPersistedExplorationRef.current === persistKey) {
      return;
    }

    lastPersistedExplorationRef.current = persistKey;
    persistExploredCells(explorationStorageKey, exploredCells);
  }, [explorationStorageKey, exploredCells]);

  const viewportRect = React.useMemo(() => {
    if (
      !deferredViewportCells ||
      !deferredCameraCenterPosition ||
      !mapWidth ||
      !mapHeight
    ) {
      return null;
    }

    const width = clamp(deferredViewportCells.width, 1, mapWidth);
    const height = clamp(deferredViewportCells.height, 1, mapHeight);
    const halfWidth = Math.floor(width / 2);
    const halfHeight = Math.floor(height / 2);
    const center = deferredCameraCenterPosition;

    const minX = clamp(center.x - halfWidth, 0, mapWidth - width);
    const minY = clamp(center.y - halfHeight, 0, mapHeight - height);

    return {
      minX,
      minY,
      maxX: minX + width - 1,
      maxY: minY + height - 1,
    };
  }, [
    deferredCameraCenterPosition,
    deferredViewportCells,
    mapHeight,
    mapWidth,
  ]);

  const playersByCellIndex = React.useMemo(() => {
    const next = new Map<number, PlayerState[]>();

    if (mapWidth <= 0 || mapHeight <= 0) {
      return next;
    }

    for (const player of deferredPlayers) {
      const idx = getCellIndex(player.position.x, player.position.y, mapWidth);
      const existing = next.get(idx);

      if (existing) existing.push(player);
      else next.set(idx, [player]);
    }

    return next;
  }, [deferredPlayers, mapHeight, mapWidth]);

  const resolveCellState = React.useCallback(
    (x: number, y: number): MiniMapCellState => {
      const cellIndex = getCellIndex(x, y, mapWidth);
      const cell = deferredGrid[cellIndex] ?? {
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
        !!localPingPoint && localPingPoint.x === x && localPingPoint.y === y;

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
      deferredGrid,
      exploredCells,
      localPingPoint,
      mapWidth,
      playersByCellIndex,
      viewportRect,
      visibleCells,
    ],
  );

  const cellStates = React.useMemo(() => {
    const next: MiniMapCellState[] = [];

    for (let y = 0; y < mapHeight; y++) {
      for (let x = 0; x < mapWidth; x++) {
        next.push(resolveCellState(x, y));
      }
    }

    return next;
  }, [mapHeight, mapWidth, resolveCellState]);

  const handleCellClick = React.useCallback(
    (state: MiniMapCellState) => {
      if (state.visibility === "unknown") return;

      const point = { x: state.cell.x, y: state.cell.y };
      setLocalPingPoint(point);
      onCellPing?.(point);
    },
    [onCellPing],
  );

  if (!mapWidth || !mapHeight || !myPlayer) {
    return null;
  }

  const effectiveSize: MiniMapSize = compact ? "small" : size;

  return (
    <aside
      className={`${styles.panel} ${compact ? styles.panelCompact : ""} ${
        effectiveSize === "large" ? styles.panelLarge : styles.panelSmall
      } ${panelClassName}`}
      aria-label="Battlefield minimap"
    >
      <div className={styles.header}>
        <div>
          <div className={styles.title}>Minimap</div>
          {!compact && (
            <div className={styles.subtitle}>
              {mapWidth}x{mapHeight} · team vision {effectiveSightRange}
            </div>
          )}
        </div>

        <div className={styles.controls}>
          {!compact && (
            <>
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
            </>
          )}

          {onCollapse && (
            <button
              type="button"
              className={`${styles.sizeButton} ${styles.collapseButton}`}
              onClick={onCollapse}
              aria-label="Hide minimap"
              title="Hide minimap"
            >
              Hide
            </button>
          )}
        </div>
      </div>

      {!compact && (
        <div className={styles.hint}>Click a known cell to focus and ping.</div>
      )}

      {!compact && (
        <div className={styles.legend}>
          <span className={styles.legendItem}>
            <span className={`${styles.legendSwatch} ${styles.youSwatch}`} />
            You
          </span>
          <span className={styles.legendItem}>
            <span className={`${styles.legendSwatch} ${styles.activeSwatch}`} />
            Turn
          </span>
          <span className={styles.legendItem}>
            <span className={`${styles.legendSwatch} ${styles.unknownSwatch}`} />
            Fog
          </span>
        </div>
      )}

      <div className={styles.grid}>
        <div
          className={styles.cells}
          style={{
            gridTemplateColumns: `repeat(${mapWidth}, minmax(0, 1fr))`,
            aspectRatio: `${mapWidth} / ${mapHeight}`,
          }}
        >
          {cellStates.map((state, index) => (
            <MiniMapCellButton
              key={`${state.cell.cell_id}:${index}`}
              state={state}
              myPlayerId={myPlayerId}
              activeUserId={activeUserId}
              onCellClick={handleCellClick}
            />
          ))}
        </div>
      </div>
    </aside>
  );
}

const MiniMapCellButton = React.memo(function MiniMapCellButton({
  state,
  myPlayerId,
  activeUserId,
  onCellClick,
}: MiniMapCellButtonProps) {
  const visiblePlayers = state.players.slice(0, MAX_RENDERED_PLAYER_MARKERS);

  return (
    <button
      type="button"
      className={getCellClassName(state)}
      title={buildCellTitle(state.cell, state.players, state.visibility)}
      onClick={() => onCellClick(state)}
      disabled={state.visibility === "unknown"}
    >
      {state.visibility === "visible" && (
        <>
          {renderPointOfInterest(state.cell)}
          {visiblePlayers.map((player, playerIndex) => {
            const markerLayout =
              PLAYER_MARKER_LAYOUT[playerIndex] ?? PLAYER_MARKER_LAYOUT[0];

            return (
              <span
                key={`player:${player.user_id}:${playerIndex}`}
                className={getPlayerMarkerClassName(
                  player,
                  myPlayerId,
                  activeUserId,
                  playerIndex > 0,
                )}
                style={{
                  left: `${markerLayout.x * 100}%`,
                  top: `${markerLayout.y * 100}%`,
                }}
              />
            );
          })}
        </>
      )}

      {state.isPinged && <span className={styles.pingMarker} />}
    </button>
  );
}, areMiniMapCellButtonPropsEqual);

export default React.memo(MiniMap, areMiniMapPropsEqual);

function areMiniMapPropsEqual(prev: MiniMapProps, next: MiniMapProps) {
  return (
    prev.grid === next.grid &&
    prev.players === next.players &&
    prev.instanceId === next.instanceId &&
    prev.mapWidth === next.mapWidth &&
    prev.mapHeight === next.mapHeight &&
    prev.myPlayerId === next.myPlayerId &&
    prev.activeUserId === next.activeUserId &&
    prev.sightRange === next.sightRange &&
    prev.compact === next.compact &&
    prev.panelClassName === next.panelClassName &&
    prev.onCellPing === next.onCellPing &&
    prev.onCollapse === next.onCollapse &&
    areViewportCellsEqual(prev.viewportCells, next.viewportCells) &&
    arePointsEqual(
      prev.cameraCenterPosition ?? null,
      next.cameraCenterPosition ?? null,
    )
  );
}
