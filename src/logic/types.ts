// src/logic/types.ts

// Определяем возможные игровые режимы.
export enum GameMode {
  PVE = "PVE",
  ONE_VS_ONE = "1v1",
  THREE_VS_THREE = "3v3",
  FIVE_VS_FIVE = "5v5",
}

// Тип для типов террейнов.
export type TerrainType = "ground" | "forest" | "mountain" | "ice" | "river" | "grass";

// Базовый интерфейс для игроков.
interface BasePlayerEntity {
  id: number;
  name: string;
  health: number;
  maxHealth: number;
  attack: number;
  defense: number;
  vision: number;
  image: string; // Для PlayerState, image как строка
  lastTurnAttacked?: number;
}

// Тип для состояния игрока.
export type PlayerState = BasePlayerEntity & {
  position: { x: number; y: number };
  energy: number;
  maxEnergy: number;
  level: number;
  experience: number;
  maxExperience: number; // Изменено на camelCase
  visionRange: number;
  inventory: Inventory;

  // Параметры для тактического боя
  speed: number;           // На сколько клеток ходит в бою
  maneuverability: number; // Сколько клеток может менять позицию при расстановке
  rangeAttack?: boolean;
  rangeDistance?: number;
};

// Базовый интерфейс для монстров.
interface BaseMonsterEntity {
  id: number;
  name: string;
  health: number;
  maxHealth: number;
  attack: number;
  defense: number;
  vision: number;
  image: Record<string, string>; // Для MonsterState, image как Record<string, string>
  lastTurnAttacked?: number;
}

// Тип для состояния монстра.
export type MonsterState = BaseMonsterEntity & {
  type: "aggressive" | "neutral";
  speed: number;          // На сколько клеток ходит в бою
  maneuverability: number;// Сколько клеток может поменять позицию перед боем
  rangeAttack?: boolean;  
  rangeDistance?: number; 
};

// Дополнительные бонусные атрибуты для предметов/артефактов.
export type BonusAttributes = {
  energy?: number; 
  maxEnergy?: number;
  visionRange?: number;
  health?: number;
  maxHealth?: number;
  attack?: number;
  defense?: number;
};

// Тип для предметов в инвентаре.
export type InventoryItem = {
  count: number;
  image: string;
  description: string;
  bonus?: BonusAttributes;
};

// Инвентарь игрока.
export type Inventory = Record<string, InventoryItem>;

// Тип для ресурса.
export type ResourceType = {
  type: string;
  difficulty: number;
  image: Record<string, string>;
  description: string;
  rarity: number;
  effect: number;
  terrains: TerrainType[];
};

// Типы действий игрока.
export enum PlayerActionEnum {
  MOVE = "move",
  ATTACK = "attack",
  COLLECT = "collect",
  USE_ITEM = "useItem",
  INTERACT = "interact",
  PASS_TURN = "passTurn",
  PICK_ARTIFACT = "pickArtifact",
  LOSE_ARTIFACT = "loseArtifact",
}

// Переходная клетка (портал, лестница).
export type TransitionTile = {
  from: TerrainType;
  to: TerrainType;
  image: string;
};

// Тип для клетки карты.
export type Cell = {
  id: number;
  x: number;
  y: number;
  terrain: TerrainType;
  resource: ResourceType | null;
  isPortal?: boolean;
  monster?: MonsterState;
  isTransition?: boolean;
  transition?: TransitionTile;
};

// Тип для клетки поля боя.
export type HexCell = {
  id: number;
  x: number;
  y: number;
  terrain: TerrainType;
  unit: PlayerState | MonsterState | null;
  walkable: boolean;
  reachable: boolean;
};

// Тип для сущности (игрок или монстр).
export type Entity = PlayerState | MonsterState;

// Тип для участников боя.
export type BattleParticipants = {
  attacker: Entity;
  defender: Entity;
};

// Тип для награды.
export type Reward = {
  experience: number;
  currency: number;
  artifact?: string;
};

// Тип для глобального состояния игры.
export type GameState = {
  mode: GameMode;
  players: PlayerState[];
  grid: Cell[];
  mapWidth: number;
  mapHeight: number;
  artifactOwner: number | null;
  portalPosition: { x: number; y: number } | null;
  instanceId: string;
  currentPlayerIndex: number;
  turnCycle: number;
  inventoryOpen: boolean;
  monstersHaveAttacked: boolean;

  // Флаг, указывающий, что мы находимся в бою (при необходимости)
  inBattle: boolean;
  battleParticipants: BattleParticipants | null;
};

export type Artifact = {
  id: number;
  name: string;
  image: string;
  description: string;
  bonus: BonusAttributes;
};
