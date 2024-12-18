// src/logic/types.ts

// Определяем возможные игровые режимы.
export type GameMode = "PVE" | "1v1" | "3v3" | "5v5";

// Тип для описания состояния монстра.
export type MonsterState = {
  id: number;
  name: string;
  type: "aggressive" | "neutral";
  health: number;
  maxHealth: number;
  attack: number;
  defense: number;
  vision: number;
  image: Record<string, string>;
  lastTurnAttacked?: number;

  // Добавляем параметры, необходимые для боя:
  speed: number;          // На сколько клеток ходит в бою
  maneuverability: number;// Сколько клеток может поменять позицию перед боем
  rangeAttack?: boolean;  
  rangeDistance?: number; 
};

// Дополнительные бонусные атрибуты для предметов/артефактов.
type BonusAttributes = {
  energy?: number; 
  maxEnergy?: number;
  visionRange?: number;
  health?: number;
  maxHealth?: number;
  attack?: number;
  defense?: number;
};

// Тип для предметов в инвентаре.
type InventoryItem = {
  count: number;
  image: string;
  description: string;
  bonus?: BonusAttributes;
};

// Инвентарь игрока.
export type Inventory = Record<string, InventoryItem>;

// Способности игрока.
export type PlayerAbilities = {
  canMove: boolean;
  canAttack: boolean;
  canCollectResources: boolean;
  canUseItems: boolean;
  canInteractWithObjects: boolean;
  canPassTurn: boolean;
  canPickArtifact: boolean;
  canLoseArtifact: boolean;
};

// Тип для состояния игрока.
export type PlayerState = {
  id: number;
  name: string;
  position: { x: number; y: number };
  energy: number;
  maxEnergy: number;
  level: number;
  experience: number;
  max_experience: number;
  visionRange: number;
  health: number;
  maxHealth: number;
  attack: number;
  defense: number;
  image: string;
  inventory: Inventory;
  abilities: PlayerAbilities;

  // Параметры для тактического боя
  speed: number;           // На сколько клеток ходит в бою
  maneuverability: number; // Сколько клеток может менять позицию при расстановке
  rangeAttack?: boolean;
  rangeDistance?: number;
};

// Тип для ресурса.
export type ResourceType = {
  type: string;
  difficulty: number;
  image: Record<string, string>;
  description: string;
  rarity: number;
  effect: number;
  terrains: string[];
};

// Типы действий игрока.
export type PlayerAction =
  | "move"
  | "attack"
  | "collect"
  | "useItem"
  | "interact"
  | "passTurn"
  | "pickArtifact"
  | "loseArtifact";

// Переходная клетка (портал, лестница).
export type TransitionTile = {
  from: string;
  to: string;
  image: string;
};

// Тип для клетки карты.
export type Cell = {
  id: number;
  x: number;
  y: number;
  terrain: string;
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
  terrain: string;
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
  inBattle?: boolean;
  battleParticipants: BattleParticipants | null;
};
