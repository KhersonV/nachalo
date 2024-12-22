// src/logic/generateMap.ts

import { PlayerState, Cell, GameMode, TerrainType } from "./types"; // Убедитесь, что путь корректен
import { resources, createRandomMonster } from "./allData"; // Объедините импорты из одного места

// Определяем возможные типы местности на карте.
const terrains: TerrainType[] = ["ground", "forest", "mountain", "ice", "river"]; // Указан тип TerrainType[]

export function generateMap(
  mode: GameMode,
  players: PlayerState[],
  width: number,
  height: number
): Cell[] {
  const total = width * height;
  const grid: Cell[] = [];
  const maxRepeats = 4;

  const getNextTerrain = (x: number, y: number, prevTerrains: TerrainType[]): TerrainType => {
    const possibleTerrains: TerrainType[] = [...terrains];

    if (y > 0) {
      const aboveTerrain = grid[(y - 1) * width + x]?.terrain;
      if (aboveTerrain) possibleTerrains.push(...Array(maxRepeats).fill(aboveTerrain));
    }

    if (x > 0) {
      const leftTerrain = grid[y * width + (x - 1)]?.terrain;
      if (leftTerrain) possibleTerrains.push(...Array(maxRepeats).fill(leftTerrain));
    }

    if (
      prevTerrains.length >= maxRepeats &&
      prevTerrains.every((t) => t === prevTerrains[0])
    ) {
      return possibleTerrains.find((t) => t !== prevTerrains[0]) || "ground";
    }

    // Выбор случайного TerrainType из possibleTerrains
    return possibleTerrains[Math.floor(Math.random() * possibleTerrains.length)];
  };

  for (let id = 0; id < total; id++) {
    const x = id % width;
    const y = Math.floor(id / width);
    const prevTerrains: TerrainType[] = [];

    if (y > 0) prevTerrains.push(grid[(y - 1) * width + x]?.terrain as TerrainType);
    if (x > 0) prevTerrains.push(grid[y * width + (x - 1)]?.terrain as TerrainType);

    const terrain: TerrainType = getNextTerrain(x, y, prevTerrains);
    const isWalkable = terrain !== "river"; // Предположим, что "river" - непроходимая местность

    const isBarrel = isWalkable && Math.random() < 0.05;
    const spawnMonster = isWalkable && Math.random() < 0.1;

    let resource: typeof resources[keyof typeof resources] | null = null;
    if (isWalkable && !isBarrel && Math.random() < 0.1) {
      const resArray = Object.values(resources);
      resource = resArray[Math.floor(Math.random() * resArray.length)];
    }

    let monster = spawnMonster ? createRandomMonster() : undefined;

    grid.push({
      id,
      x,
      y,
      terrain, // Теперь terrain строго типа TerrainType
      resource: isWalkable ? resource : null,
      monster: isWalkable ? monster : undefined,
      isPortal: false, // Инициализируем как false
    });
  }

  // Добавляем портал на карту
  const portalX = Math.floor(Math.random() * width);
  const portalY = Math.floor(Math.random() * height);
  const portalCellId = portalY * width + portalX;
  grid[portalCellId].isPortal = true;

  return grid;
}
