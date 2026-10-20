"use client";

import { useMemo } from "react";
import { axialToWorld, HEX_DIRECTIONS, hexKey, type HexCoord } from "@/utils/hexGrid";
import type { HexActor, HexEdge, HexRoute, HexSnapshot } from "@/types/HexGameTypes";

const RADIUS = 22;
const PAD = 30;
const center = (position: HexCoord) => {
  const world = axialToWorld(position, RADIUS);
  return { x: world.x, y: world.z };
};
const vertices = (position: HexCoord) => {
  const c = center(position);
  return Array.from({ length: 6 }, (_, i) => {
    const angle = (-90 + i * 60) * Math.PI / 180;
    return `${c.x + RADIUS * Math.cos(angle)},${c.y + RADIUS * Math.sin(angle)}`;
  }).join(" ");
};
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
  attackableTargets?: ReadonlySet<number>;
  busy?: boolean;
  onCellClick: (position: HexCoord) => void;
  onMonsterClick: (actor: HexActor) => void;
};

export default function HexMap({ snapshot, selected, route, selectedTargetId, attackableTargets, busy, onCellClick, onMonsterClick }: Props) {
  const bounds = useMemo(() => {
    if (!snapshot.cells.length) return { x: -100, y: -100, width: 200, height: 200 };
    const points = snapshot.cells.map(cell => center(cell.position));
    const xs = points.map(p => p.x), ys = points.map(p => p.y);
    const x = Math.min(...xs) - RADIUS - PAD, y = Math.min(...ys) - RADIUS - PAD;
    return { x, y, width: Math.max(...xs) - x + RADIUS + PAD, height: Math.max(...ys) - y + RADIUS + PAD };
  }, [snapshot.cells]);

  return <svg role="img" aria-label="HEX карта подземелья" data-testid="hex-game-map"
    viewBox={`${bounds.x} ${bounds.y} ${bounds.width} ${bounds.height}`}
    style={{ display: "block", width: "100%", height: "100%", minHeight: 460, background: "#080e16", border: "1px solid #3f5162", borderRadius: 12 }}>
    {snapshot.cells.map(cell => <polygon key={cell.cellId} data-hex-cell={`${cell.position.q},${cell.position.r}`}
      points={vertices(cell.position)} onClick={() => { if (!busy) onCellClick(cell.position); }}
      fill={cell.terrain === "rock" ? "#202b37" : cell.room ? ["#445b66", "#496252", "#5b5268"][cell.room % 3] : "#394852"}
      stroke={selected && hexKey(selected) === hexKey(cell.position) ? "#ffe16a" : "#627383"}
      strokeWidth={selected && hexKey(selected) === hexKey(cell.position) ? 3 : 0.8}
      style={{ cursor: "pointer" }}><title>{`(${cell.position.q}, ${cell.position.r}) · ${cell.terrain}`}</title></polygon>)}
    {route && snapshot.player && <polyline points={[snapshot.player.position, ...route.path].map(p => { const c = center(p); return `${c.x},${c.y}`; }).join(" ")}
      fill="none" stroke={route.affordable ? "#67f0f2" : "#ff987b"} strokeWidth={4} strokeDasharray={route.affordable ? undefined : "7 5"} pointerEvents="none" />}
    {snapshot.edges.map(edge => <line key={`${hexKey(edge.position)}:${edge.direction}`} {...wallEnds(edge)} stroke="#d8b87a" strokeWidth={4} strokeLinecap="round" pointerEvents="none" />)}
    {snapshot.cells.filter(cell => cell.resource).map(cell => { const p = center(cell.position); return <text key={`resource-${cell.cellId}`} x={p.x + 10} y={p.y - 8} pointerEvents="none" fontSize={15} fill="#f9d77b">◆</text>; })}
    {snapshot.cells.filter(cell => cell.barrel).map(cell => { const p = center(cell.position); return <text key={`barrel-${cell.cellId}`} x={p.x} y={p.y + 6} textAnchor="middle" pointerEvents="none" fontSize={17} fill="#c9955b">▣</text>; })}
    {snapshot.cells.filter(cell => cell.isPortal).map(cell => { const p = center(cell.position); return <text key={`portal-${cell.cellId}`} x={p.x} y={p.y + 7} textAnchor="middle" pointerEvents="none" fontSize={21} fill="#ac90ff">◉</text>; })}
    {snapshot.actors.map(actor => { const p = center(actor.position); const targetId = Number(actor.id.replace(/^monster-/, "")); return <g key={actor.id} role={actor.kind === "monster" ? "button" : undefined}
      data-hex-actor={actor.id} aria-label={actor.kind === "monster" ? `Монстр ${targetId}, HP ${actor.health ?? "?"}` : undefined}
      onClick={() => { if (busy) return; if (actor.kind === "monster") onMonsterClick(actor); else onCellClick(actor.position); }}
      style={{ cursor: "pointer" }}>
      <circle cx={p.x} cy={p.y} r={actor.kind === "player" ? 11 : 9} fill={actor.kind === "player" ? "#57c7ed" : "#e87565"}
        stroke={selectedTargetId === targetId && actor.kind === "monster" ? "#ffe16a" : attackableTargets?.has(targetId) && actor.kind === "monster" ? "#7beba5" : "#fff"}
        strokeWidth={selectedTargetId === targetId && actor.kind === "monster" ? 4 : 2} />
      <text x={p.x} y={p.y+4} textAnchor="middle" fontSize={11} fill="#07131d" pointerEvents="none">{actor.kind === "player" ? "P" : "M"}</text>
      {actor.kind === "monster" && <text x={p.x} y={p.y+20} textAnchor="middle" fontSize={10} fill="#fff" stroke="#17202b" strokeWidth={2} paintOrder="stroke" pointerEvents="none">{actor.health}/{actor.maxHealth}</text>}
      <title>{`${actor.kind} · HP ${actor.health ?? "?"}/${actor.maxHealth ?? "?"}`}</title>
    </g>; })}
  </svg>;
}
