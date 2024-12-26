

//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//************************************************* src/logic/allData.ts ******************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************

//resourceData
import { ResourceType, Artifact } from "@/logic/types";


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
    rarity: 0.3,
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


//terrainData

export const terrainData: Record<
  string,
  { image: string; defenseModifier: number; transition_to?: Record<string, string> }
> = {
  ground: {
    image: "./main_tails/ground.webp",
    defenseModifier: 0,
    transition_to: {
      grass: "./mixed_tails/ground_to/ground_grass_transition.webp",
      river: "./mixed_tails/ground_to/ground_river_transition.webp",
      forest: "./mixed_tails/ground_to/ground_forest_transition.webp",
      mountain: "./mixed_tails/ground_to/ground_mountain_transition.webp",
      ice: "./mixed_tails/ground_to/ground_ice_transition.webp",
    },
  },
  forest: {
    image: "./main_tails/forest.webp",
    defenseModifier: 1,
    transition_to: {
      grass: "./mixed_tails/forest_to/forest_grass_transition.webp",
      ground: "./mixed_tails/forest_to/forest_ground_transition.webp",
      mountain: "./mixed_tails/forest_to/forest_mountain_transition.webp",
      river: "./mixed_tails/forest_to/forest_river_transition.webp",
      ice: "./mixed_tails/forest_to/forest_ice_transition.webp",
    },
  },
  mountain: {
    image: "./main_tails/mountain.webp",
    defenseModifier: 2,
    transition_to: {
      grass: "./mixed_tails/mountain_to/mountain_grass_transition.webp",
      forest: "./mixed_tails/mountain_to/mountain_forest_transition.webp",
      ground: "./mixed_tails/mountain_to/mountain_ground_transition.webp",
      river: "./mixed_tails/mountain_to/mountain_river_transition.webp",
      ice: "./mixed_tails/mountain_to/mountain_ice_transition.webp",
    },
  },
  ice: {
    image: "./main_tails/ice.webp",
    defenseModifier: -1,
    transition_to: {
      river: "./mixed_tails/ice_to/ice_river_transition.webp",
      grass: "./mixed_tails/ice_to/ice_grass_transition.webp",
      mountain: "./mixed_tails/ice_to/ice_mountain_transition.webp",
      ground: "./mixed_tails/ice_to/ice_ground_transition.webp",
      forest: "./mixed_tails/ice_to/ice_forest_transition.webp",
    },
  },
  river: {
    image: "./main_tails/river.webp",
    defenseModifier: -2,
    transition_to: {
      grass: "./mixed_tails/river_to/river_grass_transition.webp",
      ground: "./mixed_tails/river_to/river_ground_transition.webp",
      forest: "./mixed_tails/river_to/river_forest_transition.webp",
      ice: "./mixed_tails/river_to/river_ice_transition.webp",
      mountain: "./mixed_tails/river_to/river_mountain_transition.webp",
    },
  },
  grass: {
    image: "./main_tails/grass.webp",
    defenseModifier: 0,
    transition_to: {
      river: "./mixed_tails/grass_to/grass_river_transition.webp",
      ice: "./mixed_tails/grass_to/grass_ice_transition.webp",
      ground: "./mixed_tails/grass_to/grass_ground_transition.webp",
      forest: "./mixed_tails/grass_to/grass_forest_transition.webp",
      mountain: "./mixed_tails/grass_to/grass_mountain_transition.webp",
    },
  },
};


//monsterData

import { MonsterState } from "./types";

let uniqueId = 0;
function getUniqueId(): number {
  return uniqueId++;
}

export const allMonsters: MonsterState[] = [
  {
    id: getUniqueId(),
    name: "Goblin",
    type: "aggressive",
    health: 20,
    maxHealth: 20,
    attack: 25,
    defense: 1,
    vision: 1,
    speed: 1,
    maneuverability: 1,
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
    health: 40,
    maxHealth: 40,
    attack: 10,
    defense: 2,
    vision: 1,
    speed: 1,
    maneuverability: 1,
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
    health: 50,
    maxHealth: 50,
    attack: 15,
    defense: 5,
    vision: 1,
    speed: 1,
    maneuverability: 1,
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
    health: 80,
    maxHealth: 80,
    attack: 40,
    defense: 10,
    vision: 1,
    speed: 1,
    maneuverability: 1,
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

//artifactData

export const artifacts: Artifact[] = [
  {
    id: 1,
    name: "Sword of the King",
    image: "/artifacts/knight-sword.webp",
    description: "Меч рыцаря, выкованный в королевской кузне.",
    bonus: { attack: 5 },
  },
  {
    id: 2,
    name: "Guardian Shield",
    image: "/artifacts/guardian-shield.webp",
    description: "Надежный щит стража, отражает удары мощью древних рун.",
    bonus: { defense: 4, maxHealth: 10 },
  },
  {
    id: 3,
    name: "Ring of Wisdom",
    image: "/artifacts/ring-of-wisdom.webp",
    description: "Кольцо мудрости, усиливающее дальновидность и решимость.",
    bonus: { visionRange: 2, energy: 5 },
  },
  {
    id: 4,
    name: "Boots of Stealth",
    image: "/artifacts/boots_of_stealth.webp",
    description: "Сапоги скрытности, позволяющие незаметно передвигаться.",
    bonus: { defense: 2, energy: 10 },
  },
  {
    id: 5,
    name: "Dragon Eye",
    image: "/artifacts/dragon-eye.webp",
    description: "Глаз дракона, придаёт владельцу сокрушительную силу.",
    bonus: { attack: 8, maxHealth: 5 },
  },
  {
    id: 6,
    name: "Fire Amulet",
    image: "/artifacts/fire-amulet.webp",
    description: "Амулет огня, наполненный магической энергией древних.",
    bonus: { attack: 3, energy: 15 },
  },
  {
    id: 7,
    name: "Titan Breastplate",
    image: "/artifacts/titan-breastplate.webp",
    description: "Нагрудник титана, повышает защиту и выносливость.",
    bonus: { defense: 6, maxHealth: 15 },
  },
  {
    id: 8,
    name: "Crown of Enlightenment",
    image: "/artifacts/crown-of-enlightenment.webp",
    description: "Корона просветления, помогает видеть слабости врага.",
    bonus: { visionRange: 3, defense: 2 },
  },
  {
    id: 9,
    name: "Berserker Axe",
    image: "/artifacts/berserker-axe.webp",
    description: "Боевой топор берсерка, увеличивает физическую мощь.",
    bonus: { attack: 7, health: 10 },
  },
  {
    id: 10,
    name: "Gloves of Precision",
    image: "/artifacts/gloves-of-precision.webp",
    description: "Перчатки меткости, усиливают точность ударов.",
    bonus: { attack: 2, energy: 5, defense: 1 },
  },
];