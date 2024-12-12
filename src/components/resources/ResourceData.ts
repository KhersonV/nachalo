export type ResourceType = {
  type: string;
  difficulty: number;
  image: Record<string, string>;
  description: string;
  rarity: number;
  effect: number;
  terrains: string[];
};

export const resources: { [key: string]: ResourceType } = {
  food: {
    type: "food",
    difficulty: 1,
    image: {
      ground: "/food-ground.webp",
      forest: "/food-forest.webp",
    },
    description: "Еда для выживания.",
    rarity: 0.1,
    effect: 20,
    terrains: ["ground, forest"],
  },
  wood: {
    type: "wood",
    difficulty: 1,
    image: {
      forest: "/wood-forest.webp",
      ice: "/wood-ice.webp",
    },
    description: "Древесина для строительства.",
    rarity: 0.1,
    effect: 1,
    terrains: ["forest", "ice"],
  },
  water: {
    type: "water",
    difficulty: 1,
    image: {
      water: "/water-ground.webp",
      ice: "/water-ice.webp",
    },
    description: "Вода для питья.",
    rarity: 0.1,
    effect: 5,
    terrains: ["ground", "ice"],
  },
  stone: {
    type: "stone",
    difficulty: 2,
    image: {
      mountain: "/stone-mountain.webp",
      ice: "/stone-ice.webp",
    },
    description: "Камень для строительства.",
    rarity: 0.01,
    effect: 1,
    terrains: ["mountain", "ice"],
  },
  iron: {
    type: "iron",
    difficulty: 2,
    image: {
      mountain: "/iron-mountain.webp",
      ice: "/iron-ice.webp",
    },
    description: "Железо для инструментов.",
    rarity: 0.01,
    effect: 1,
    terrains: ["mountain", "ice"],
  },
  barrbel: {
    type: "barrbel",
    difficulty: 2,
    image:{
      ground: "",
      ice: "",
    },
    description: "Бочка с ресурсом, артефактом или монстром.",
    rarity: 0.01,
    effect: 0,
    terrains: ["ground", "ice"],
  },
};

export function getResourceImage(resource: ResourceType, terrain: string): string {
  return resource.image[terrain];
}

export const getResource = (type: string): ResourceType | null => {
  return resources[type] || null;
};
