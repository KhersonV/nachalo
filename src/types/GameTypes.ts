
// ==============================
// src/types/GameTypes.ts
// ==============================

export type BonusAttributes = {
  energy?: number;
  maxEnergy?: number;
  visionRange?: number;
  health?: number;
  maxHealth?: number;
  attack?: number;
  defense?: number;
};

// Определяем, как приходит каждый элемент из бэка
export type RawInventoryItem = {
  item_type: "resource" | "artifact";
  item_id: number;
  name: string;             // раньше было item_name
  item_count: number;
  image: string;            // соответствует image_url на бэке
  description: string;      // соответствует item_description на бэке
  bonus?: BonusAttributes;
  effect?: Record<string, number>;
};

// Типизированный инвентарь уже с разделением
export type Inventory = {
  resources: Record<string, RawInventoryItem>;
  artifacts: Record<string, RawInventoryItem>;
};

export type PlayerState = {
  user_id: number;
  name: string;
  health: number;
  maxHealth: number;
  attack: number;
  defense: number;
  vision: number;
  image: string;
  lastTurnAttacked?: number;
  position: { x: number; y: number };
  energy: number;
  maxEnergy: number;
  level: number;
  experience: number;
  maxExperience: number;
  visionRange: number;
  inventory: Inventory;
  speed: number;
  maneuverability: number;
  rangeAttack?: boolean;
  rangeDistance?: number;
};

export type MonsterType = {
  id: number;
  db_instance_id: number;  
  name: string;
  type: string;
  health: number;
  maxHealth: number;
  attack: number;
  defense: number;
  speed: number;
  maneuverability: number;
  vision: number;
  image: string;
  created_at: string;
};

export type ResourceType = {
  id: number;
  type: string;
  description: string;
  effect: Record<string, number>;
  image: string;
};

export interface Cell {
  cell_id: number;
  x: number;
  y: number;
  tileCode: number;
  resource: ResourceType | null;
  barbel: ResourceType | null;
  monster: MonsterType | null;
  isPortal: boolean;
  isPlayer: boolean;
}

export type GameState = {
  instanceId: string;
  mode: string;
  grid: Cell[];
  mapWidth: number;
  mapHeight: number;
  players: PlayerState[];
  active_user: number;
  turnNumber: number;
  isMapLoaded: boolean;
};

// Здесь перечисляем возможные направления
export type Dir = "up" | "down" | "left" | "right";
