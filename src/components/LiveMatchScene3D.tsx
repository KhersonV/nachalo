"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useSelector } from "react-redux";
import type { RootState } from "@/store";
import type { Cell, PlayerState } from "@/types/GameTypes";
import {
  buildExplorationStorageKey, buildTeamExplorationBaseKey, collectVisibleCellIndices,
  getCellIndex, isSameTeamPlayer, loadExploredCells, persistExploredCells,
} from "@/utils/fogOfWar";
import { adaptRealMap } from "@/utils/realMapAdapter";
import { normalizeAvatarPath } from "@/utils/normalizeAvatarPath";
import { useCombatPresentationPlayback } from "@/hooks/useCombatPresentationPlayback";
import NachaloScene3D from "./NachaloScene3D";
import type { RangerCue, RangerModel, RangerStatus } from "./AnimatedRanger";
import combatMetadata from "@/data/rangerCombat.json";

type Props = {
  instanceId: string;
  grid: Cell[];
  mapWidth: number;
  mapHeight: number;
  players: PlayerState[];
  myPlayer: PlayerState;
  focusPoint?: { x: number; y: number } | null;
  onCellClick: (cell: Cell) => void;
  onPlayerClick: (player: PlayerState) => void;
};

function seedFromInstance(instanceId: string) {
  let seed = 2166136261;
  for (const char of instanceId) seed = Math.imul(seed ^ char.charCodeAt(0), 16777619);
  return seed >>> 0;
}

/** Reads the same authoritative Redux snapshot and fog rules as Classic. */
export default function LiveMatchScene3D({ instanceId, grid, mapWidth, mapHeight, players,
  myPlayer, focusPoint, onCellClick, onPlayerClick }: Props) {
  // Keep the shared combat queue advancing while Classic's animation layer is unmounted.
  const [model, setModel] = useState<RangerModel>("animated");
  const [status, setStatus] = useState<RangerStatus | null>(null);
  const [loadError, setLoadError] = useState(false);
  const isRanger = myPlayer.characterType?.toLowerCase() === "ranger" ||
    normalizeAvatarPath(myPlayer.image).startsWith("/ranger/");
  const playback = useCombatPresentationPlayback(isRanger && model === "animated" ? {
    rangedActorId: myPlayer.user_id, rangedReleaseMs: combatMetadata.attackReleaseSeconds * 1000,
  } : undefined);
  useEffect(() => {
    if (status?.model === "animated" && status.failed) { setLoadError(true); setModel("legacy"); }
  }, [status]);
  const combat = useSelector((state: RootState) => state.combatPresentation);
  const rangerCues = useMemo<RangerCue[]>(() => {
    const exchange = combat.queue.find(item => item.exchangeId === combat.activeExchangeId);
    if (!exchange) return [];
    const id = myPlayer.user_id;
    const isPlayer = (ref: { type: string; id: number }) => ref.type === "player" && ref.id === id;
    if (exchange.steps.some(step => step.kind === "death" && isPlayer(step.target)))
      return [{ id: `${exchange.exchangeId}:death`, clip: "Death" }];
    if (model === "legacy") {
      if (exchange.steps.some(step => step.kind !== "death" && isPlayer(step.target) && "damage" in step && step.damage > 0))
        return [{ id: `${exchange.exchangeId}:hit`, clip: "Hit" }];
      if (exchange.attackStyle === "ranged" && exchange.steps.some(step => "source" in step && step.source && isPlayer(step.source)))
        return [{ id: `${exchange.exchangeId}:attack`, clip: "Attack_Bow", target: exchange.targetSnapshot?.position }];
      return [];
    }
    const cues: RangerCue[] = [];
    if (exchange.attackStyle === "ranged" && exchange.attackerType === "player" && exchange.attackerId === id) {
      const motion = playback.activeMotions.find(m => m.exchangeId === exchange.exchangeId && m.id.endsWith('motion:attack'));
      const projectile = playback.activeEffects.find(e => e.id === `${exchange.exchangeId}:projectile:attack`);
      if (motion) cues.push({ id: `${exchange.exchangeId}:attack`, clip: "Attack_Bow",
        target: exchange.targetSnapshot?.position, startedAt: motion.startMs / 1000, releaseAt: projectile ? projectile.startMs / 1000 : undefined });
    }
    exchange.steps.forEach((step,index) => {
      if (step.kind !== "death" && isPlayer(step.target) && "damage" in step && step.damage > 0) {
        const flash = playback.activeEffects.find(e => e.id === `${exchange.exchangeId}:flash:${index}`);
        if (flash) cues.push({ id: `${exchange.exchangeId}:hit:${index}`, clip: "Hit", startedAt: flash.startMs/1000 });
      }
    });
    return cues;
  }, [combat, model, myPlayer.user_id, playback.activeMotions, playback.activeEffects]);
  const storageKey = buildExplorationStorageKey(
    buildTeamExplorationBaseKey(instanceId, myPlayer.user_id, myPlayer.group_id),
    mapWidth, mapHeight);
  const [explored, setExplored] = useState(() => loadExploredCells(storageKey));
  const visibilitySources = useMemo(() => players.filter(player =>
    isSameTeamPlayer(player, myPlayer.user_id, myPlayer.group_id)).map(player => ({
    position: player.position, sightRange: player.sightRange ?? myPlayer.sightRange ?? 3,
  })), [players, myPlayer.user_id, myPlayer.group_id, myPlayer.sightRange]);
  const visible = useMemo(() => collectVisibleCellIndices(visibilitySources, mapWidth, mapHeight),
    [visibilitySources, mapWidth, mapHeight]);
  const known = useMemo(() => new Set([...explored, ...visible]), [explored, visible]);

  useEffect(() => {
    setExplored(previous => {
      const next = new Set([...previous, ...visible]);
      return next.size === previous.size ? previous : next;
    });
  }, [visible]);
  useEffect(() => persistExploredCells(storageKey, explored), [storageKey, explored]);

  // A match's stone topology is fixed. Retain the first server snapshot so
  // WebSocket cell/player/turn updates only change sprites and fog in Three.js.
  const terrainRef = useRef<Cell[] | null>(null);
  if (!terrainRef.current && grid.length === mapWidth * mapHeight) terrainRef.current = grid;
  const seed = useMemo(() => seedFromInstance(instanceId), [instanceId]);
  const view = useMemo(() => {
    try { return adaptRealMap({ map: grid, map_width: mapWidth, map_height: mapHeight, players }, visible, known); }
    catch { return null; }
  }, [grid, mapWidth, mapHeight, players, visible, known]);

  if (!view || !terrainRef.current) return <p role="status">3D карта ожидает корректное состояние матча.</p>;

  return <div>
    {isRanger && <div role="group" aria-label="Модель Ranger" style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap", color: "white" }}>
      {(["animated", "legacy"] as const).map(choice => <button key={choice} type="button"
        aria-pressed={model === choice} onClick={() => { setModel(choice); setStatus(null); setLoadError(false); }}
        style={{ padding: "6px 12px", border: "1px solid #789", borderRadius: 6, color: "white",
          background: model === choice ? "#245c73" : "#172334" }}>{choice === "animated" ? "Animated" : "Legacy"}</button>)}
      <output aria-label="Анимация Ranger" data-clip={status?.clip} data-clip-time={status?.clipTime}
        data-arrow-visible={status?.arrowVisible}>{status?.ready ? `${status.clip} · Walk ×${status.walkTimeScale.toFixed(2)}` : "Загрузка модели…"}</output>
      {loadError && <span role="status">Animated недоступен — выбран Legacy.</span>}
    </div>}
    <NachaloScene3D grid={terrainRef.current} mapWidth={mapWidth} mapHeight={mapHeight}
    objects={view.objects} renderAllTerrain materialSeed={seed} heroPosition={myPlayer.position}
    ranger={isRanger ? {
      userId: myPlayer.user_id, position: myPlayer.position, cues: rangerCues, model,
      defeated: myPlayer.health <= 0 || rangerCues.some(cue => cue.clip === "Death"),
    } : undefined}
    onRangerStatus={isRanger ? setStatus : undefined} combatEffects={isRanger ? playback.activeEffects.filter(effect =>
      !(effect.kind === "deathFade" && effect.actor.type === "player" && effect.actor.id === myPlayer.user_id && status?.ready)) : undefined}
    cameraFocus={focusPoint}
    visibleCells={visible} exploredCells={known}
    onCellClick={point => {
      if (!visible.has(getCellIndex(point.x, point.y, mapWidth))) return;
      const current = grid.find(cell => cell.x === point.x && cell.y === point.y);
      if (current) onCellClick(current);
    }}
    onPlayerClick={userId => {
      const player = players.find(item => item.user_id === userId);
      if (player && visible.has(getCellIndex(player.position.x, player.position.y, mapWidth))) onPlayerClick(player);
    }} />
  </div>;
}
