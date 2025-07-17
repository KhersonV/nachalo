
//==============================
// src/utils/buildPlayerMap.ts
//==============================


import { PlayerState } from "@/types/GameTypes";

export function buildPlayerMap(players: PlayerState[]): Map<string, PlayerState> {
  const map = new Map<string, PlayerState>();
  for (const player of players) {
    const key = `${player.position.x}-${player.position.y}`;
    map.set(key, player);
  }
  return map;
}
