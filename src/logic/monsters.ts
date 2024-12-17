// src/logic/monsters.ts

import { Cell, PlayerState } from "./types";
import { Action } from "./actions";

export function aggressiveMonstersAttack(
  state: {
    players: PlayerState[];
    grid: Cell[] | null;
    artifactOwner: number | null;
    mode: string;
    turnCycle: number;
    monstersHaveAttacked: boolean;
  },
  dispatch: React.Dispatch<Action>
): { newState: typeof state; instanceFinished: boolean } {
  console.log(`aggressiveMonstersAttack вызвана для turnCycle=${state.turnCycle}`);

  if (!state.grid) return { newState: state, instanceFinished: false };
  if (state.monstersHaveAttacked) {
    console.log("Монстры уже атаковали в этом turnCycle.");
    return { newState: state, instanceFinished: false };
  }

  let instanceFinished = false;

  state.grid.forEach((cell) => {
    if (cell.monster && cell.monster.type === "aggressive") {
      state.players.forEach((player) => {
        const distance = Math.abs(player.position.x - cell.x) + Math.abs(player.position.y - cell.y);
        if (cell.monster && distance <= cell.monster.vision && player.health > 0) {
          const damage = Math.max(0, cell.monster.attack - player.defense);

          console.log(`Монстр ID=${cell.monster.id} атакует игрока ID=${player.id} с уроном ${damage}`);

          dispatch({
            type: "ATTACK",
            payload: {
              attackerId: cell.monster.id,
              targetId: player.id,
              damage,
              targetType: "player",
            },
          });

          if (player.health - damage <= 0) {
            console.log(`Игрок ID=${player.id} был убит монстром ID=${cell.monster.id}`);
            dispatch({
              type: "PLAYER_DIED",
              payload: { playerId: player.id },
            });
            instanceFinished = true; // Можно считать конец инстанса
          }
        }
      });
    }
  });

  // Устанавливаем флаг, что монстры уже атаковали в этом круге
  dispatch({
    type: "SET_MONSTERS_HAVE_ATTACKED",
    payload: { monstersHaveAttacked: true },
  });

  return { newState: state, instanceFinished };
}
