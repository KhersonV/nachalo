import { PlayerState, Cell } from "../components/GameContext";
import { resources } from "../components/resources/ResourceData";
import { GameMode } from "./matchmaking";
import { createRandomMonster } from "./monsterData";

const terrains = ["water", "ground", "mountain", "forest", "ice"];

export function generateMap(mode: GameMode, players: PlayerState[], width: number, height: number): Cell[] {
  const total = width * height;
  const grid: Cell[] = Array.from({ length: total }, (_, id) => {
    const x = id % width;
    const y = Math.floor(id / width);
    const terrain = terrains[Math.floor(Math.random() * terrains.length)];
    const isBarrel = Math.random() < 0.05;
    const spawnMonster = Math.random() < 0.02;
    let resource = null;
    if (!isBarrel && Math.random() < 0.1) {
      const resArray = Object.values(resources);
      resource = resArray[Math.floor(Math.random() * resArray.length)];
    }

    let monster = spawnMonster ? createRandomMonster() : undefined;

    return { id, x, y, terrain, resource, isBarrel, monster };
  });

  // Добавим портал
  const portalX = Math.floor(Math.random()*width);
  const portalY = Math.floor(Math.random()*height);
  const portalCellId = portalY*width+portalX;
  grid[portalCellId].isPortal = true;

  return grid;
}
