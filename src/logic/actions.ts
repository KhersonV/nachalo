// src/logic/actions.ts

import { GameMode, PlayerState, MonsterState, Cell} from "./types";

// Определяем типы действий
export type Action =
  | { type: 'INITIALIZE_GAME'; payload: { mode: GameMode; instanceId: string; players: PlayerState[] } }
  | { type: 'SET_INSTANCE_ID'; payload: { instanceId: string } }
  | { type: 'MOVE_PLAYER'; payload: { playerId: number; newPosition: { x: number; y: number } } }
  | { type: 'ATTACK'; payload: { attackerId: number; targetId: number; damage: number } }
  | { type: 'COLLECT_RESOURCE'; payload: { playerId: number; resourceType: string } }
  | { type: 'USE_ITEM'; payload: { playerId: number; itemType: string } }
  | { type: 'PASS_TURN' }
  | { type: 'PICK_ARTIFACT'; payload: { playerId: number } }
  | { type: 'LOSE_ARTIFACT'; payload: { playerId: number } }
  | { type: 'ADD_ITEM'; payload: { playerId: number; itemType: string; description: string; image: string; bonus?: Record<string, number> } }
  | { type: 'REMOVE_RESOURCE'; payload: { cellId: number } }
  | { type: 'ADD_MONSTER'; payload: { cellId: number; monster: MonsterState } }
  | { type: 'TRY_EXIT_PORTAL'; payload: { playerId: number } }
  | { type: 'PLAYER_DIED'; payload: { playerId: number } }
  | { type: 'FINALIZE_INSTANCE'; payload: { instanceId: string; players: PlayerState[] } }
  | { type: 'SET_GRID'; payload: { grid: Cell[] } }
  | { type: 'AWARD_REWARD'; payload: { playerId: number; reward: Reward } }
  | { type: 'TOGGLE_INVENTORY' }
  ;
  
// Дополнительные типы
export type Reward = {
  experience: number;
  currency: number;
};
