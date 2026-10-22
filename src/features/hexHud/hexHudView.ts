import type { HexActor, HexAttackPreview, HexCell, HexPlayer, HexRoute, HexSnapshot } from "@/types/HexGameTypes";
import type { HexCoord } from "@/utils/hexGrid";

export type HexHudView = {
  hero?: HexPlayer;
  target?: HexActor;
  cell?: HexCell;
  preview?: HexAttackPreview;
  route?: HexRoute;
  isMyTurn: boolean;
  remaining: number;
  turnSeconds: number;
  finished: boolean;
  inventoryCount: number;
};

/** Presentation only. The server still decides whether every command is legal. */
export function toHexHudView(snapshot: HexSnapshot, selected: HexCoord | null, targetId: number | null, targetKind: "player" | "monster", preview: HexAttackPreview | null, route: HexRoute | null, now: number, unavailable = false): HexHudView {
  const hero = snapshot.player;
  const cell = selected ? snapshot.cells.find(item => item.position.q === selected.q && item.position.r === selected.r && item.visibility !== "unexplored") : undefined;
  const target = targetId === null ? undefined : snapshot.actors.find(actor => actor.id === `${targetKind}-${targetId}`);
  const finished = !!snapshot.finished || !!snapshot.result || !hero || hero.health <= 0 || unavailable;
  return {
    hero, cell, target,
    preview: target && preview?.targetId === targetId ? preview : undefined,
    route: cell && route ? route : undefined,
    isMyTurn: !finished && snapshot.activeUser === hero?.userId,
    remaining: snapshot.turnDeadline && !snapshot.finished ? Math.max(0, Math.ceil((Date.parse(snapshot.turnDeadline) - now) / 1000)) : 0,
    turnSeconds: snapshot.turnSeconds || 60,
    finished,
    inventoryCount: Object.values(hero?.inventory ?? {}).reduce((total, item) => total + Math.max(0, Number(item.item_count) || 0), 0),
  };
}
