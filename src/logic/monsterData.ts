//monsterData.ts

import { get } from "http";
import { MonsterState } from "../logic/types";

let uniqueId = 0;
function getUniqueId(): number {
  return uniqueId++;
}

export const allMonsters: MonsterState[] = [
  {
    id: getUniqueId(),
    name: "Goblin",
    type: "aggressive",
    hp: 20,
    maxHp: 20,
    attack: 25,
    defense: 1,
    vision: 3,
    image: {
      ground: "/monsters/ground/goblin_ground.webp",
      forest: "/monsters/forest/goblin_forest.webp",
      grass: "/monsters/grass/goblin_grass.webp",
      ice: "/monsters/ice/goblin_ice.webp",
      mountain: "/monsters/mountain/goblin_mountain.webp",
    },
  },
  {
    id: getUniqueId(),
    name: "Goblins",
    type: "neutral",
    hp: 40,
    maxHp: 40,
    attack: 50,
    defense: 2,
    vision: 1,
    image: {
      ground: "/monsters/ground/goblins_ground.webp",
      forest: "/monsters/forest/goblins_forest.webp",
      grass: "/monsters/grass/goblins_grass.webp",
      ice: "/monsters/ice/goblins_ice.webp",
      mountain: "/monsters/mountain/goblins_mountain.webp",
    },
  },
  {
    id: getUniqueId(),
    name: "Orc",
    type: "aggressive",
    hp: 50,
    maxHp: 50,
    attack: 30,
    defense: 5,
    vision: 2,
    image: {
      ground: "/monsters/ground/orc_ground.webp",
      forest: "/monsters/forest/orc_forest.webp",
      grass: "/monsters/grass/orc_grass.webp",
      ice: "/monsters/ice/orc_ice.webp",
      mountain: "/monsters/mountain/orc_mountain.webp",
    },
  },
  {
    id: getUniqueId(),
    name: "Troll",
    type: "neutral",
    hp: 80,
    maxHp: 80,
    attack: 40,
    defense: 10,
    vision: 1,
    image: {
      ground: "/monsters/ground/troll_ground.webp",
      forest: "/monsters/forest/troll_forest.webp",
      grass: "/monsters/grass/troll_grass.webp",
      ice: "/monsters/ice/troll_ice.webp",
      mountain: "/monsters/mountain/troll_mountain.webp",
    },
  },
];



export function createRandomMonster(): MonsterState {
  const randomIndex = Math.floor(Math.random() * allMonsters.length);
  const basemonster = allMonsters[randomIndex];
  return {
    ...basemonster,
    id: getUniqueId(),
  };
}
