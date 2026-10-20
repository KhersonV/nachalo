"use client";

import { useMemo } from "react";
import { axialToWorld, HEX_DIRECTIONS, hexKey, type HexCoord } from "@/utils/hexGrid";
import { heroArt, monsterArt, resourceArt } from "@/utils/hexArt";
import { useHexCamera } from "@/hooks/useHexCamera";
import type { HexActor, HexCombatIndicator, HexEdge, HexRoute, HexSnapshot } from "@/types/HexGameTypes";
import styles from "@/styles/HexMap.module.css";
import type { HexGameMessages } from "@/i18n/hexGame";

const RADIUS = 46;
const center = (position: HexCoord) => {
  const world = axialToWorld(position, RADIUS);
  return { x: world.x, y: world.z };
};
const vertexPoints = (position: HexCoord) => {
  const p = center(position);
  return Array.from({ length: 6 }, (_, i) => {
    const angle = (-90 + i * 60) * Math.PI / 180;
    return { x: p.x + RADIUS * Math.cos(angle), y: p.y + RADIUS * Math.sin(angle) };
  });
};
const vertices = (position: HexCoord) => vertexPoints(position).map(p => `${p.x},${p.y}`).join(" ");
// For pointy-top axial directions E, NE, NW, W, SW, SE.
const boundaryCorners = [[1, 2], [0, 1], [5, 0], [4, 5], [3, 4], [2, 3]] as const;
const wallEnds = (edge: HexEdge) => {
  const a = center(edge.position);
  const d = HEX_DIRECTIONS[edge.direction];
  const b = center({ q: edge.position.q + d.q, r: edge.position.r + d.r });
  const dx = b.x - a.x, dy = b.y - a.y;
  const length = Math.hypot(dx, dy);
  const mx = (a.x + b.x) / 2, my = (a.y + b.y) / 2;
  const tx = -dy / length * RADIUS / 2, ty = dx / length * RADIUS / 2;
  return { x1: mx - tx, y1: my - ty, x2: mx + tx, y2: my + ty };
};

type Props = {
  snapshot: HexSnapshot;
  selected?: HexCoord | null;
  route?: HexRoute | null;
  selectedTargetId?: number | null;
	selectedTargetKind?: "player" | "monster";
  attackableTargets?: ReadonlySet<number>;
  indicators?: HexCombatIndicator[];
  busy?: boolean;
  onCellClick: (position: HexCoord) => void;
  onMonsterClick: (actor: HexActor) => void;
  labels?: HexGameMessages;
};

export default function HexMap({ snapshot, selected, route, selectedTargetId, selectedTargetKind, attackableTargets, indicators = [], busy, onCellClick, onMonsterClick, labels }: Props) {
  const hero = useMemo(() => center(snapshot.player?.position ?? { q: 0, r: 0 }), [snapshot.player?.position.q, snapshot.player?.position.r]);
  const camera = useHexCamera(hero);
  const routeCells = useMemo(() => new Set(route?.path.map(hexKey) ?? []), [route]);
  const boundaryPath = useMemo(() => {
    const positions = new Set(snapshot.cells.map(cell => hexKey(cell.position)));
    return snapshot.cells.flatMap(cell => {
      const corners = vertexPoints(cell.position);
      return HEX_DIRECTIONS.flatMap((direction, index) => {
        const neighbor = { q: cell.position.q + direction.q, r: cell.position.r + direction.r };
        if (positions.has(hexKey(neighbor))) return [];
        const [a, b] = boundaryCorners[index];
        return [`M${corners[a].x},${corners[a].y}L${corners[b].x},${corners[b].y}`];
      });
    }).join(" ");
  }, [snapshot.cells]);
  const ownPosition = snapshot.player?.position && hexKey(snapshot.player.position);
  const sprite = heroArt(snapshot.player?.characterType);

  return <div className={styles.viewport} data-testid="hex-game-map" data-camera-zoom={camera.camera.zoom.toFixed(3)} data-camera-x={camera.camera.x.toFixed(1)} data-camera-y={camera.camera.y.toFixed(1)}>
    <svg ref={camera.svgRef} role="img" aria-label={labels?.map ?? "HEX dungeon map"} className={styles.map} viewBox={camera.viewBox}
      data-panning={camera.panning} onWheel={camera.onWheel} onPointerDown={camera.onPointerDown} onPointerMove={camera.onPointerMove}
      onPointerUp={camera.onPointerUp} onPointerCancel={camera.onPointerUp} onClickCapture={camera.onClickCapture}>
      <defs>
        <pattern id="hex-floor" patternUnits="userSpaceOnUse" width="184" height="184"><image href="/assets/nachalo/catacombs/floor_stone.webp" width="184" height="184" /></pattern>
        <pattern id="hex-rock" patternUnits="userSpaceOnUse" width="190" height="190"><image href="/assets/nachalo/catacombs/wall_base.webp" width="190" height="190" /></pattern>
        <clipPath id="hex-portrait" clipPathUnits="objectBoundingBox"><circle cx=".5" cy=".5" r=".5" /></clipPath>
      </defs>
      <g className={styles.tiles}>
        {snapshot.cells.map(cell => {
          const key = hexKey(cell.position);
          const floor = cell.terrain === "floor";
          const active = selected && hexKey(selected) === key;
          const interactive = !!cell.resource;
          const visibility = cell.visibility ?? "visible";
          return <g key={cell.cellId} data-hex-visibility={visibility}>
            <polygon data-hex-cell={key} points={vertices(cell.position)} fill={visibility === "unexplored" ? "#05090e" : floor ? "url(#hex-floor)" : "url(#hex-rock)"}
              className={visibility === "unexplored" ? styles.unknownCell : floor ? styles.floor : styles.rock} onClick={() => { if (!busy && visibility !== "unexplored") onCellClick(cell.position); }}>
              <title>{visibility === "unexplored" ? labels?.unexplored ?? "Unexplored" : `${cell.terrain === "floor" ? labels?.floor ?? "floor" : labels?.rock ?? "rock"} · ${key}`}</title>
            </polygon>
            {floor && visibility === "visible" && <polygon points={vertices(cell.position)} className={styles.floorTint} pointerEvents="none" />}
            {visibility === "explored" && <polygon points={vertices(cell.position)} className={styles.exploredFog} pointerEvents="none" />}
            {interactive && visibility === "visible" && <polygon points={vertices(cell.position)} className={styles.interactiveCell} pointerEvents="none" />}
            {ownPosition === key && <polygon points={vertices(cell.position)} className={styles.heroCell} pointerEvents="none" />}
            {routeCells.has(key) && <polygon points={vertices(cell.position)} className={route?.affordable ? styles.routeCell : styles.routeBlocked} pointerEvents="none" />}
            {active && <polygon points={vertices(cell.position)} className={styles.selectedCell} pointerEvents="none" />}
          </g>;
        })}
      </g>
      {route && snapshot.player && <g pointerEvents="none">
        <polyline className={styles.routeLineShadow} points={[snapshot.player.position, ...route.path].map(p => { const c = center(p); return `${c.x},${c.y}`; }).join(" ")} />
        <polyline className={route.affordable ? styles.routeLine : styles.routeLineBlocked} points={[snapshot.player.position, ...route.path].map(p => { const c = center(p); return `${c.x},${c.y}`; }).join(" ")} />
      </g>}
      <g pointerEvents="none">
        {snapshot.edges.map(edge => <g key={`${hexKey(edge.position)}:${edge.direction}`}>
          <line {...wallEnds(edge)} className={styles.wallBase} />
          <line {...wallEnds(edge)} className={styles.wallStone} />
          <line {...wallEnds(edge)} className={styles.wallLight} />
        </g>)}
      </g>
      <g className={styles.objects}>
        {snapshot.cells.filter(cell => cell.resource || cell.barrel || cell.isPortal || cell.structureType).map(cell => {
          const p = center(cell.position);
          const resource = cell.resource && resourceArt(cell.resource);
          return <g key={`object-${cell.cellId}`} data-hex-object={hexKey(cell.position)} className={styles.object}
            onClick={event => { event.stopPropagation(); if (!busy) onCellClick(cell.position); }}>
            <circle cx={p.x} cy={p.y} r="29" fill="transparent" />
            {cell.isPortal && <><circle cx={p.x} cy={p.y} r="26" className={styles.portalRing} /><text x={p.x} y={p.y + 11} textAnchor="middle" className={styles.portalIcon}>◉</text></>}
            {cell.structureType && <><image href={`/Forge-items/${cell.structureType}.png`} x={p.x - 22} y={p.y - 26} width="44" height="44" /><text x={p.x} y={p.y + 23} textAnchor="middle" className={styles.healthText}>{cell.isUnderConstruction ? `🚧 ${cell.constructionTurnsLeft}` : `HP ${cell.structureHealth}`}</text></>}
            {cell.barrel && <image href="/main_resources/barrel.webp" x={p.x - 21} y={p.y - 26} width="42" height="45" preserveAspectRatio="xMidYMid meet" />}
            {cell.resource && (resource
              ? <image href={resource} x={p.x - 21} y={p.y - 23} width="42" height="42" preserveAspectRatio="xMidYMid meet" />
              : <><circle cx={p.x} cy={p.y} r="17" className={styles.fallbackResource} /><text x={p.x} y={p.y + 7} textAnchor="middle" className={styles.fallbackSymbol}>◆</text></>)}
            <title>{cell.structureType ? `${cell.structureType} · HP ${cell.structureHealth} · owner ${cell.structureOwnerUserId}` : cell.resource?.type ?? (cell.barrel ? labels?.barrel ?? "Barrel" : labels?.portal ?? "Portal")}</title>
          </g>;
        })}
      </g>
      <g className={styles.actors}>
        {snapshot.actors.map(actor => {
          const p = center(actor.position);
          const targetId = actor.kind === "player" ? actor.userId ?? 0 : Number(actor.id.replace(/^monster-/, ""));
          const self = actor.kind === "player" && actor.userId === snapshot.player?.userId;
          const image = actor.kind === "monster" ? monsterArt(actor) : self ? sprite : heroArt(undefined);
          const selectedMonster = !self && selectedTargetKind === actor.kind && selectedTargetId === targetId;
          return <g key={actor.id} role={!self ? "button" : undefined}
            data-hex-actor={actor.id} aria-label={actor.kind === "monster" ? `${labels?.monster ?? "Monster"} ${targetId}, HP ${actor.health ?? "?"}` : undefined}
            className={styles.actor} onClick={event => { event.stopPropagation(); if (busy) return; if (!self) onMonsterClick(actor); else onCellClick(actor.position); }}>
            <circle cx={p.x} cy={p.y} r="30" fill="transparent" />
            <ellipse cx={p.x} cy={p.y + 21} rx="24" ry="9" className={styles.shadow} pointerEvents="none" />
            {self && <circle cx={p.x} cy={p.y} r="27" className={styles.selfRing} pointerEvents="none" />}
            {selectedMonster && <circle cx={p.x} cy={p.y} r="30" className={styles.targetRing} pointerEvents="none" />}
            {actor.kind === "monster" && attackableTargets?.has(targetId) && !selectedMonster && <circle cx={p.x} cy={p.y} r="29" className={styles.attackableRing} pointerEvents="none" />}
            {image && actor.kind === "monster" && <image href={image} x={p.x - 24} y={p.y - 29} width="48" height="48" preserveAspectRatio="xMidYMid slice" clipPath="url(#hex-portrait)" />}
            {image && actor.kind === "player" && <image href={image} x={p.x - 27} y={p.y - 43} width="54" height="64" preserveAspectRatio="xMidYMid meet" />}
            {!image && <><circle cx={p.x} cy={p.y - 4} r="21" className={styles.fallbackMonster} /><text x={p.x} y={p.y + 3} textAnchor="middle" className={styles.fallbackSymbol}>{actor.name?.slice(0, 1) ?? "M"}</text></>}
            {!self && <g pointerEvents="none">
              <rect x={p.x - 25} y={p.y + 24} width="50" height="7" rx="3" className={styles.healthTrack} />
              <rect x={p.x - 24} y={p.y + 25} width={Math.max(0, 48 * (actor.health ?? 0) / Math.max(1, actor.maxHealth ?? 1))} height="5" rx="2" className={styles.healthFill} />
              <text x={p.x} y={p.y + 43} textAnchor="middle" className={styles.healthText}>{actor.health}/{actor.maxHealth}</text>
            </g>}
            {actor.kind === "player" && !self && <text x={p.x} y={p.y + 39} textAnchor="middle" className={styles.playerName}>{actor.name ?? `${labels?.player ?? "Player"} ${actor.userId}`}</text>}
            <title>{`${actor.name ?? actor.kind} · HP ${actor.health ?? "?"}/${actor.maxHealth ?? "?"}`}</title>
          </g>;
        })}
      </g>
      <g className={styles.indicators} pointerEvents="none" aria-hidden="true">
        {indicators.map(indicator => {
          const p = center(indicator.position);
          return <text key={indicator.id} x={p.x} y={p.y - 35 - Math.min(indicator.lane, 4) * 17} textAnchor="middle" className={`${styles.indicator} ${styles[indicator.tone]}`}>{indicator.text}</text>;
        })}
      </g>
      <g pointerEvents="none" aria-hidden="true">
        <path data-hex-boundary="true" d={boundaryPath} className={styles.boundaryGlow} vectorEffect="non-scaling-stroke" />
        <path d={boundaryPath} className={styles.boundaryGold} vectorEffect="non-scaling-stroke" />
      </g>
    </svg>
    <div className={styles.cameraControls} aria-label={labels?.camera ?? "Map camera"}>
      <button type="button" onClick={() => camera.zoomBy(1.2)} aria-label={labels?.zoomIn ?? "Zoom in"}>+</button>
      <button type="button" onClick={camera.recenter}>{labels?.recenter ?? "To hero"}</button>
      <button type="button" onClick={() => camera.zoomBy(1 / 1.2)} aria-label={labels?.zoomOut ?? "Zoom out"}>−</button>
    </div>
  </div>;
}
