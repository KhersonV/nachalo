
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//************************************************* src/logic/generateMap.ts **************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************


import { PlayerState, Cell, GameMode, TerrainType } from "./types";
import { resources, createRandomMonster } from "./allData";

const terrains: TerrainType[] = ["ground", "forest", "mountain", "ice", "river"];

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

    return possibleTerrains[Math.floor(Math.random() * possibleTerrains.length)];
  };

  for (let id = 0; id < total; id++) {
    const x = id % width;
    const y = Math.floor(id / width);
    const prevTerrains: TerrainType[] = [];

    if (y > 0) {
      prevTerrains.push(grid[(y - 1) * width + x]?.terrain as TerrainType);
    }
    if (x > 0) {
      prevTerrains.push(grid[y * width + (x - 1)]?.terrain as TerrainType);
    }

    const terrain: TerrainType = getNextTerrain(x, y, prevTerrains);
    const isWalkable = terrain !== "river";

    // ---------------------------
    // 1) Определяем, есть ли бочка
    // ---------------------------
    const BARREL_SPAWN_CHANCE = 0.1; // 10% (пример)
    const isBarrel = isWalkable && Math.random() < BARREL_SPAWN_CHANCE;

    // ---------------------------
    // 2) Определяем, есть ли "обычный" ресурс
    // ---------------------------
    // (Чтобы не затирать бочку, делаем это во "фторую" проверку)
    let resource: typeof resources[keyof typeof resources] | null = null;
    if (isBarrel) {
      // Если выпала бочка
      resource = resources["barrbel"];
    } else if (isWalkable && Math.random() < 0.1) {
      // Иначе с 10% возьмём случайный ресурс
      const resArray = Object.values(resources);
      resource = resArray[Math.floor(Math.random() * resArray.length)];
    }

    // ---------------------------
    // 3) Определяем, есть ли монстр
    // ---------------------------
    const spawnMonster = isWalkable && Math.random() < 0.05;
    const monster = spawnMonster ? createRandomMonster() : undefined;

    // Формируем клетку
    grid.push({
      id,
      x,
      y,
      terrain,
      resource: isWalkable ? resource : null,
      monster: isWalkable ? monster : undefined,
      isPortal: false, // потом можно задать "true" для одной клетки
    });
  }

  // Добавляем портал на карту
  const portalX = Math.floor(Math.random() * width);
  const portalY = Math.floor(Math.random() * height);
  const portalCellId = portalY * width + portalX;
  grid[portalCellId].isPortal = true;

  return grid;
}
