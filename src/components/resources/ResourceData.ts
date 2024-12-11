export type ResourceType = {
  type: string;
  difficulty: number;
  image: string;
  description: string;
  rarity: number;
  effect: number;
  terrains: string[];
};

export const resources: { [key: string]: ResourceType } = {
  food: {
    type: "food",
    difficulty: 1,
    image: "/food.webp",
    description: "Еда для выживания.",
    rarity: 0.1,
    effect: 20,
    terrains: ["ground"],
  },
  wood: {
    type: "wood",
    difficulty: 1,
    image: "/wood.webp",
    description: "Древесина для строительства.",
    rarity: 0.1,
    effect: 1,
    terrains: ["forest", "ice"],
  },
  water: {
    type: "water",
    difficulty: 1,
    image: "/water.webp",
    description: "Вода для питья.",
    rarity: 0.1,
    effect: 5,
    terrains: ["water", "ice"],
  },
  stone: {
    type: "stone",
    difficulty: 2,
    image: "/stone.webp",
    description: "Камень для строительства.",
    rarity: 0.01,
    effect: 1,
    terrains: ["mountain", "ice"],
  },
  iron: {
    type: "iron",
    difficulty: 2,
    image: "/iron.webp",
    description: "Железо для инструментов.",
    rarity: 0.01,
    effect: 1,
    terrains: ["mountain", "ice"],
  },
};

export const getResource = (type: string): ResourceType | null => {
  return resources[type] || null;
};
