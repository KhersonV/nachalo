
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

export type InventoryItem = {
  name?: string;
  item_count: number;
  image: string;
  description: string;
  bonus?: BonusAttributes;
  effect?: Record<string, number>;
};

export type Inventory = {
  resources: Record<string, InventoryItem>;
  artifacts: Record<string, InventoryItem>;
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
};

export type Action =
  | {
    type: "SET_MATCH_DATA";
    payload: {
      instanceId: string;
      mode: string;
      grid: Cell[];
      mapWidth: number;
      mapHeight: number;
      players: PlayerState[];
      active_user: number;
      turnNumber: number;
    };
  }
  | {
    type: "MOVE_PLAYER";
    payload: { userId: number; newPosition: { x: number; y: number } };
  }
  | {
    type: "SET_ACTIVE_USER";
    payload: { active_user: number; turnNumber: number; energy?: number };
  }
  | {
    type: "UPDATE_CELL";
    payload: { updatedCell: Cell };
  }
  | {
    type: "UPDATE_PLAYER";
    payload: { player: PlayerState };
  }
  |
  {
    type: "COMBAT_EXCHANGE"; payload: {
      attacker: { id: number; new_hp: number }
      target: { id: number; new_hp: number }
    }
  }
  | {
    type: "UPDATE_INVENTORY";
    payload: { userId: number; inventory: Inventory };
  }
  | {
    type: "RESET_STATE";
  };

// Здесь перечисляем возможные направления
export type Dir = "up" | "down" | "left" | "right";
