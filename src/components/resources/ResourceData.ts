//ResourceData.ts

import { ResourceType } from "@/logic/types";


export const resources: { [key: string]: ResourceType } = {
  food: {
    type: "food",
    difficulty: 1,
    image: {
      ground: "/food-ground.webp",
      forest: "/food-forest.webp",
      grass: "/food-grass.webp",
      ice: "/food-ice.webp",
      mountain: "/food-mountain.webp",
    },
    description: "Еда для выживания.",
    rarity: 0.1,
    effect: 20, // Пополнение здоровья
    terrains: ["ground", "forest", "grass", "ice", "mountain"],
  },
  wood: {
    type: "wood",
    difficulty: 1,
    image: {
      forest: "/wood-forest.webp",
      ice: "/wood-ice.webp",
      grass: "/wood-grass.webp",
      ground: "/wood-ground.webp",
      mountain: "/wood-mountain.webp",
    },
    description: "Древесина для строительства.",
    rarity: 0.1,
    effect: 1, // Используется для крафта
    terrains: ["forest", "ice", "grass", "ground", "mountain"],
  },
  water: {
    type: "water",
    difficulty: 1,
    image: {
      ground: "/water-ground.webp",
      ice: "/water-ice.webp",
      grass: "/water-grass.webp",
      forest: "/water-forest.webp",
      mountain: "/water-mountain.webp",
    },
    description: "Вода для питья.",
    rarity: 0.1,
    effect: 5, // Пополнение энергии
    terrains: ["ground", "ice", "grass", "forest", "mountain"],
  },
  stone: {
    type: "stone",
    difficulty: 2,
    image: {
      mountain: "/stone-mountain.webp",
      ice: "/stone-ice.webp",
      grass: "/stone-grass.webp",
      ground: "/stone-ground.webp",
      forest: "/stone-forest.webp",
    },
    description: "Камень для строительства.",
    rarity: 0.01,
    effect: 1, // Используется для крафта
    terrains: ["mountain", "ice", "grass", "ground", "forest"],
  },
  iron: {
    type: "iron",
    difficulty: 2,
    image: {
      mountain: "/iron-mountain.webp",
      ice: "/iron-ice.webp",
      grass: "/iron-grass.webp",
      ground: "/iron-ground.webp",
      forest: "/iron-forest.webp",
    },
    description: "Железо для инструментов.",
    rarity: 0.01,
    effect: 1, // Используется для улучшений
    terrains: ["mountain", "ice", "grass", "ground", "forest"],
  },
  barrbel: {
    type: "barrbel",
    difficulty: 1,
    image: {
      ground: "/barrbel-ground.webp",
      ice: "/barrbel-ice.webp",
      grass: "/barrbel-grass.webp",
      forest: "/barrbel-forest.webp",
      mountain: "/barrbel-mountain.webp",
    },
    description: "Бочка с ресурсом, артефактом или монстром.",
    rarity: 0.01,
    effect: 0, // Эффект зависит от содержимого
    terrains: ["ground", "ice", "grass", "forest", "mountain"],
  },
};

// Получение изображения ресурса в зависимости от местности
export function getResourceImage(resource: ResourceType, terrain: string): string {
  const basePath = "./resources_on_tile"; // Базовый путь к папке с ресурсами
  const terrainPath = `${basePath}/${terrain}`; // Формируем путь на основе террейна
  const imagePath = resource.image[terrain]; // Получаем имя изображения для текущего террейна

  // Если путь существует, возвращаем полный путь
  return imagePath ? `${terrainPath}${imagePath}` : "/default-resource.webp";
}

// Получение ресурса по типу
export const getResource = (type: string): ResourceType | null => {
  return resources[type] || null;
};
