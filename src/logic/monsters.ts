// src/logic/monsters.ts

import { Cell, PlayerState } from "../logic/types";
import { Action } from "./actions";

export function aggressiveMonstersAttack(state: { players: PlayerState[]; grid: Cell[] | null; artifactOwner: number | null; mode: string }, dispatch: React.Dispatch<Action>): { newState: typeof state, instanceFinished: boolean } {
  if (!state.grid) return { newState: state, instanceFinished: false };
  let instanceFinished = false;

  state.grid.forEach((cell) => {
    if (cell.monster && cell.monster.type === 'aggressive') {
      state.players.forEach((player) => {
        const distance = Math.abs(player.position.x - cell.x) + Math.abs(player.position.y - cell.y);
        if (cell.monster && distance <= cell.monster.vision && player.health > 0) {
          const damage = Math.max(0, cell.monster.attack - player.defense);
          dispatch({ type: 'ATTACK', payload: { attackerId: cell.monster.id, targetId: player.id, damage } });
          if (player.health - damage <= 0) {
            dispatch({ type: 'PLAYER_DIED', payload: { playerId: player.id } });
            instanceFinished = true; // Можно считать конец инстанса
          }
        }
      });
    }
  });

  return { newState: state, instanceFinished };
}
