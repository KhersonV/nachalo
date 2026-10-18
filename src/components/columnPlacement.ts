import type { Cell } from "@/types/GameTypes";
import { getCellIndex } from "@/utils/fogOfWar";
import { TILE } from "@/utils/realMapAdapter";

export function cellHash(x: number, y: number, seed: number) {
  let hash = Math.imul(x + 17, 73856093) ^ Math.imul(y + 29, 19349663) ^ Math.imul(seed, 83492791);
  hash = Math.imul(hash ^ (hash >>> 16), 2246822507);
  return (hash ^ (hash >>> 13)) >>> 0;
}

/** A column changes the appearance of an existing internal obstacle only. */
export function selectColumnCells(grid: readonly Cell[], width: number, height: number,
  known: ReadonlySet<number>, seed: number): ReadonlySet<number> {
  const cells = new Map<number, Cell>();
  for (const cell of grid) {
    const index = getCellIndex(cell.x, cell.y, width);
    if (known.has(index)) cells.set(index, cell);
  }
  const selected = new Set<number>();
  const directions = [[0, -1], [1, 0], [0, 1], [-1, 0]] as const;
  for (const [index, cell] of cells) {
    const { x, y } = cell;
    if (cell.tileCode !== TILE.obstacle || x <= 0 || y <= 0 || x >= width - 1 || y >= height - 1 ||
        cell.isPortal || cell.isPlayer || cell.resource || cell.barbel || cell.monster || cell.structure_type) continue;
    // All immediate topology must already be known. A newly discovered area
    // cannot change an earlier choice based on secret neighboring cells.
    let neighborhoodKnown = true;
    for (let dy = -1; dy <= 1; dy++) for (let dx = -1; dx <= 1; dx++) {
      if (!cells.has(getCellIndex(x + dx, y + dy, width))) neighborhoodKnown = false;
    }
    if (!neighborhoodKnown) continue;
    const blocked = directions.filter(([dx, dy]) => {
      const neighbor = cells.get(getCellIndex(x + dx, y + dy, width));
      return neighbor?.tileCode === TILE.obstacle || neighbor?.tileCode === TILE.border;
    });
    if (blocked.length > 1) continue; // retain corners, runs, T and cross junctions
    if (blocked.length === 1) {
      const [dx, dy] = blocked[0];
      const nextX = x + 2 * dx, nextY = y + 2 * dy;
      const next = nextX >= 0 && nextY >= 0 && nextX < width && nextY < height
        ? cells.get(getCellIndex(nextX, nextY, width)) : undefined;
      // A true wall end must still attach to a second internal wall tile.
      if (next?.tileCode !== TILE.obstacle || cellHash(x, y, seed) % 2 !== 0) continue;
    } else if (cellHash(x, y, seed) % 3 === 0) continue;
    selected.add(index);
  }
  return selected;
}
