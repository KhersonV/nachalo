//==============================
// src/hooks/usePlayerInCell.ts
//==============================

import { useSelector } from "react-redux";
import type { RootState } from "@/store";
import type { Cell } from "@/types/GameTypes";

export function usePlayerInCell(cell: Cell) {
  const players = useSelector((state: RootState) => state.game.players);
  if (!cell.isPlayer) return null;
  return players.find((p) => p.position.x === cell.x && p.position.y === cell.y) || null;
}
