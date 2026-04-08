//==============================
// src/utils/getPlayerInCell.ts
//==============================

import { Cell, PlayerState } from "@/types/GameTypes";

export function getPlayerInCell(cell: Cell, players: PlayerState[]): PlayerState | null {
  if (!cell.isPlayer) return null;
  return players.find((p) => p.position.x === cell.x && p.position.y === cell.y) || null;
}
