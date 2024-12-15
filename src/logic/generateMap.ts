// generateMap.ts

// Импортируем необходимые типы и данные.
import { PlayerState, Cell, GameMode } from "../logic/types";
import { resources } from "../components/resources/ResourceData"; // Данные о ресурсах.
import { createRandomMonster } from "./monsterData"; // Функция для создания случайного монстра.

// Определяем возможные типы местности на карте.
const terrains = ["ground", "forest", "mountain", "ice"];

/**
 * Функция для генерации карты игры.
 * @param mode - Текущий игровой режим (например, "PVE", "1v1").
 * @param players - Список игроков в текущей игре.
 * @param width - Ширина карты (в клетках).
 * @param height - Высота карты (в клетках).
 * @returns Массив клеток, представляющих карту.
 */
export function generateMap(
  mode: GameMode,
  players: PlayerState[],
  width: number,
  height: number
): Cell[] {
  const total = width * height; // Общее количество клеток на карте.
  const grid: Cell[] = []; // Массив для хранения клеток карты.
  const maxRepeats = 4; // Максимальное количество повторений одной местности подряд.

  /**
   * Функция для определения следующего типа местности.
   * @param x - Координата X клетки.
   * @param y - Координата Y клетки.
   * @param prevTerrains - Список предыдущих типов местности.
   * @returns Тип местности для текущей клетки.
   */
  const getNextTerrain = (x: number, y: number, prevTerrains: string[]): string => {
    const possibleTerrains = [...terrains]; // Список возможных типов местности.

    // Добавляем больше шансов для соседних типов местности.
    if (y > 0) {
      const aboveTerrain = grid[(y - 1) * width + x]?.terrain;
      if (aboveTerrain) possibleTerrains.push(...Array(maxRepeats).fill(aboveTerrain));
    }

    if (x > 0) {
      const leftTerrain = grid[y * width + (x - 1)]?.terrain;
      if (leftTerrain) possibleTerrains.push(...Array(maxRepeats).fill(leftTerrain));
    }

    // Если предыдущая местность повторяется слишком много раз, выбираем другую.
    if (
      prevTerrains.length >= maxRepeats &&
      prevTerrains.every((t) => t === prevTerrains[0])
    ) {
      return possibleTerrains.find((t) => t !== prevTerrains[0]) || "ground";
    }

    // Выбираем случайный тип местности из возможных.
    return possibleTerrains[Math.floor(Math.random() * possibleTerrains.length)];
  };

  // Генерация клеток карты.
  for (let id = 0; id < total; id++) {
    const x = id % width; // Координата X клетки.
    const y = Math.floor(id / width); // Координата Y клетки.
    const prevTerrains = []; // Массив для хранения соседних типов местности.

    // Определяем тип местности сверху.
    if (y > 0) prevTerrains.push(grid[(y - 1) * width + x]?.terrain);

    // Определяем тип местности слева.
    if (x > 0) prevTerrains.push(grid[y * width + (x - 1)]?.terrain);

    const terrain = getNextTerrain(x, y, prevTerrains); // Выбираем тип местности.
    const isWalkable = terrain !== "river"; // Проверяем, можно ли ходить по этой местности.

    // С вероятностью 5% добавляем бочку.
    const isBarrel = isWalkable && Math.random() < 0.05;

    // С вероятностью 10% добавляем монстра.
    const spawnMonster = isWalkable && Math.random() < 0.1;

    // С вероятностью 10% добавляем ресурс (если это не бочка).
    let resource = null;
    if (isWalkable && !isBarrel && Math.random() < 0.1) {
      const resArray = Object.values(resources); // Список всех ресурсов.
      resource = resArray[Math.floor(Math.random() * resArray.length)]; // Выбираем случайный ресурс.
    }

    // Если нужно, создаём монстра.
    let monster = spawnMonster ? createRandomMonster() : undefined;

    // Добавляем клетку в массив карты.
    grid.push({
      id, // Уникальный идентификатор клетки.
      x, // Координата X.
      y, // Координата Y.
      terrain, // Тип местности.
      resource: isWalkable ? resource : null, // Ресурс, если клетка проходима.
      monster: isWalkable ? monster : undefined, // Монстр, если клетка проходима.
    });
  }

  // Добавляем портал на карту.
  const portalX = Math.floor(Math.random() * width); // Случайная координата X для портала.
  const portalY = Math.floor(Math.random() * height); // Случайная координата Y для портала.
  const portalCellId = portalY * width + portalX; // Уникальный ID клетки с порталом.
  grid[portalCellId].isPortal = true; // Устанавливаем флаг портала.

  return grid; // Возвращаем сгенерированную карту.
}
