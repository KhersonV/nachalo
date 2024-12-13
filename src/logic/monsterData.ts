//monsterData.ts

import { MonsterState } from "../logic/types";

export const allMonsters: MonsterState[] = [
  {
    name: "Goblin",
    type: "aggressive",
    hp: 20,
    maxHp: 20,
    attack: 25,
    defense: 1,
    vision: 3,
    image: {
      ground: "/goblin_ground.webp",
      ice: "/goblin_ice.webp",
      grass: "/goblin_grass.webp",
      forest: "/goblin_forest.webp",
      mountain: "/goblin_mountain.webp",
    },
  },
  {
    name: "Goblins",
    type: "neutral",
    hp: 40,
    maxHp: 40,
    attack: 50,
    defense: 2,
    vision: 1,
    image: {
      ground: "/goblins_ground.webp",
      ice: "/goblins_ice.webp",
      grass: "/goblins_grass.webp",
      forest: "/goblins_forest.webp",
      mountain: "/goblins_mountain.webp",
    },
  },
  {
    name: "Orc",
    type: "aggressive",
    hp: 50,
    maxHp: 50,
    attack: 30,
    defense: 5,
    vision: 2,
    image: {
      ground: "/orc_ground.webp",
      ice: "/orc_ice.webp",
      grass: "/orc_grass.webp",
      forest: "/orc_forest.webp",
      mountain: "/orc_mountain.webp",
    },
  },
  {
    name: "Troll",
    type: "neutral",
    hp: 80,
    maxHp: 80,
    attack: 40,
    defense: 10,
    vision: 1,
    image: {
      ground: "/troll_ground.webp",
      ice: "/troll_ice.webp",
      grass: "/troll_grass.webp",
      forest: "/troll_forest.webp",
      mountain: "/troll_mountain.webp",
    },
  },
];

export function createRandomMonster(): MonsterState {
  const randomIndex = Math.floor(Math.random() * allMonsters.length);
  return allMonsters[randomIndex];
}
