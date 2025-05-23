
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
    payload: { instanceId: string; userId: number; newPosition: { x: number; y: number } };
  }
  | {
    type: "SET_ACTIVE_USER";
    payload: { instanceId: string; active_user: number; turnNumber: number; energy?: number };
  }
  | {
    type: "UPDATE_CELL";
    payload: { instanceId: string; updatedCell: Cell };
  }
  | {
    type: "UPDATE_PLAYER";
    payload: { instanceId: string; player: PlayerState };
  }
  |
  {
    type: "COMBAT_EXCHANGE"; payload: {
      instanceId: string;
      attacker: { id: number; new_hp: number }
      target: { id: number; new_hp: number }
    }
  }
  | {
    type: "UPDATE_INVENTORY";
    payload: {instanceId: string; userId: number; inventory: Inventory };
  }
  | {
    type: "RESET_STATE";
  }
  
  | {
      type: "BARREL_RESOURCE";
      payload: {
        instanceId: string;
        updatedCell: Cell;
        updatedPlayer: PlayerState;
      };
    }
  | {
      type: "BARREL_ARTIFACT";
      payload: {
        instanceId: string;
        updatedCell: Cell;
        updatedPlayer: PlayerState;
      };
    }

  | { type: "PLAYER_DEFEATED"; payload: { userId: number } }
  | {
      type: "TURN_PASSED";
      payload: {
        active_user: number;   // новый текущий игрок
      };
    };

// Здесь перечисляем возможные направления
export type Dir = "up" | "down" | "left" | "right";
