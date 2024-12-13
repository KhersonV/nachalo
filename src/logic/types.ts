// src/logic/types.ts
export type GameMode = "PVE" | "1v1" | "3v3" | "5v5";

export type MonsterState = {
  name: string;
  type: "aggressive" | "neutral";
  hp: number;
  maxHp: number;
  attack: number;
  defense: number;
  vision: number;
  image: Record<string, string>;
};

export type ResourceType = {
  type: string;
  difficulty: number;
  image: Record<string, string>;
  description: string;
  rarity: number;
  effect: number;
  terrains: string[];
};

// Новые типы для расширения действий и переходных тайлов
export type PlayerAction =
  | "move"
  | "attack"
  | "collect"
  | "useItem"
  | "interact"
  | "passTurn"
  | "pickArtifact"
  | "loseArtifact";

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

export type TransitionTile = {
  from: string;
  to: string;
  image: string;
};

export type PlayerState = {
  id: number;
  name: string;
  position: { x: number; y: number };
  energy: number;
  maxEnergy: number;
  level: number;
  expirience: number;
  max_expirience: number;
  visionRange: number;
  health: number;
  maxHealth: number;
  attack: number;
  defense: number;
  image: string;
  inventory: Record<string, { count: number; image: string; description: string }>;
  abilities: PlayerAbilities; // Добавили способности
};

export type Cell = {
  id: number;
  x: number;
  y: number;
  terrain: string;
  resource: ResourceType | null;
  //isBarrel?: boolean;
  isPortal?: boolean;
  monster?: MonsterState;
  isTransition?: boolean;    // Новый флаг, если это переходный тайл
  transition?: TransitionTile; // Данные о переходе, если переходный
};

export type GameState = {
  mode: GameMode;
  players: PlayerState[];
  grid: Cell[] | null;
  mapWidth: number;
  mapHeight: number;
  artifactOwner: number | null;
  portalPosition: { x: number; y: number } | null;
  instanceId: string;
  currentPlayerIndex: number;
};
