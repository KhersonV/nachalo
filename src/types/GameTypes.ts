
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
    count: number;
    image: string;
    description: string;
    bonus?: BonusAttributes;
  };
  
  export type Inventory = {
    resources: Record<string, InventoryItem>;
    artifacts: Record<string, InventoryItem>;
  };
  
  export type PlayerState = {
    id: number;
    name: string;
    health: number;
    maxHealth: number;
    attack: number;
    defense: number;
    vision: number;
    image: string;
    colorClass: string;
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
    id: number;
    x: number;
    y: number;
    tileCode: number;
    resource: ResourceType | null;
    monster?: MonsterType | null;
    isPortal?: boolean;
  }
  
  export type GameState = {
    instanceId: string;
    mode: string;
    grid: Cell[];
    mapWidth: number;
    mapHeight: number;
    players: PlayerState[];
    currentPlayerId: number;
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
          activePlayer: number;
          turnNumber: number;
        };
      }
    | {
        type: "MOVE_PLAYER";
        payload: { playerId: number; newPosition: { x: number; y: number } };
      }
    | {
        type: "SET_ACTIVE_PLAYER";
        payload: { activePlayer: number; turnNumber: number };
      }
    | {
        type: "RESET_STATE";
      };
  