// src/logic/utils.ts

import { Cell } from "./types";

export function checkForDuplicateMonsters(grid: Cell[]): void {
  const monsterIds = new Set<number>();
  grid.forEach(cell => {
    if (cell.monster) {
      if (monsterIds.has(cell.monster.id)) {
        console.error(`Дублирование монстра с ID=${cell.monster.id} в клетке (${cell.x}, ${cell.y})`);
      } else {
        monsterIds.add(cell.monster.id);
      }
    }
  });
}
