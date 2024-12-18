//generateBattlefield.ts

import { HexCell, PlayerState, MonsterState } from './types';

export function generateBattlefield(
  width: number,
  height: number,
  attacker: PlayerState | MonsterState,
  defender: PlayerState | MonsterState
): HexCell[] {
  const battlefield: HexCell[] = [];

  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      const id = y * width + x;
      const cell: HexCell = {
        id,
        x,
        y,
        terrain: 'grass',
        unit: null,
        walkable: true,
        reachable: false
      };
      battlefield.push(cell);
    }
  }

  // Размещаем атакующего слева, защитника справа
  const attackerX = 1;
  const attackerY = Math.floor(height / 2);
  const attackerIndex = attackerY * width + attackerX;

  const defenderX = width - 2;
  const defenderY = Math.floor(height / 2);
  const defenderIndex = defenderY * width + defenderX;

  battlefield[attackerIndex].unit = attacker;
  battlefield[defenderIndex].unit = defender;

  return battlefield;
}
