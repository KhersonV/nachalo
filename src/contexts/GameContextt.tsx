// ==============================
// src/contexts/GameContextt.tsx
// ==============================

"use client";

import React, { createContext, useContext, useReducer, useEffect, Dispatch, useState } from "react";
import { useRouter } from "next/navigation";
import { useGameSocket } from "../hooks/useGameSocket";

// Типы для бонусов и инвентаря
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

// Тип игрока, используемый на клиенте
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

// Тип монстра
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

// Тип ресурса
export type ResourceType = {
  id: number;
  type: string;
  description: string;
  effect: Record<string, number>;
  image: string;
};

// Тип клетки – мы добавляем поле cell_id вместо id
export type Cell = {
  cell_id: number;
  x: number;
  y: number;
  tileCode: number;
  resource: ResourceType | null;
  monster: MonsterType | null;
  isPortal?: boolean;
  // isPlayer можно добавить, если нужно
  isPlayer?: boolean;
};

// Состояние игры
export type GameState = {
  instanceId: string;
  mode: string;
  grid: Cell[]; // фиксированная карта, полученная с сервера
  mapWidth: number;
  mapHeight: number;
  players: PlayerState[];
  currentPlayerId: number;
  turnNumber: number;
};

// Действия редьюсера
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

const initialState: GameState = {
  instanceId: "",
  mode: "",
  grid: [],
  mapWidth: 0,
  mapHeight: 0,
  players: [],
  currentPlayerId: 0,
  turnNumber: 1,
};

// Создаем контекст
const GameContext = createContext<{ state: GameState; dispatch: Dispatch<Action> } | undefined>(undefined);

// Редьюсер обновляет состояние игры по действиям
export function gameReducer(state: GameState, action: Action): GameState {
  switch (action.type) {
    case "SET_MATCH_DATA":
      return {
        ...state,
        instanceId: action.payload.instanceId,
        mode: action.payload.mode,
        grid: action.payload.grid,
        mapWidth: action.payload.mapWidth,
        mapHeight: action.payload.mapHeight,
        players: action.payload.players,
        currentPlayerId: action.payload.activePlayer,
        turnNumber: action.payload.turnNumber || state.turnNumber,
      };
    case "MOVE_PLAYER":
      return {
        ...state,
        players: state.players.map((p) =>
          p.id === action.payload.playerId ? { ...p, position: action.payload.newPosition } : p
        ),
      };
    case "SET_ACTIVE_PLAYER":
      return {
        ...state,
        currentPlayerId: action.payload.activePlayer,
        turnNumber: action.payload.turnNumber,
      };
    case "RESET_STATE":
      return { ...initialState };
    default:
      return state;
  }
}

type GameProviderProps = {
  instanceId: string;
  children: React.ReactNode;
};

// Функция GameProvider инициализирует состояние, загружает данные (ресурсы, монстров, матч) и подключает WebSocket.
export function GameProvider({ instanceId, children }: GameProviderProps) {
  const router = useRouter();
  const [state, dispatch] = useReducer(gameReducer, { ...initialState, instanceId });
  const [resources, setResources] = useState<ResourceType[]>([]);
  const [monsters, setMonsters] = useState<MonsterType[]>([]);

  // Загружаем ресурсы и монстров с сервера при изменении instanceId.
  useEffect(() => {
    console.log("instanceId изменился на:", instanceId);
    dispatch({ type: "RESET_STATE" });

    async function fetchResources() {
      try {
        const res = await fetch("http://localhost:8001/api/resources", { cache: "no-store" });
        if (!res.ok) {
          console.error("Ошибка загрузки ресурсов", await res.text());
          return;
        }
        const data = await res.json();
        console.log("Полученные ресурсы:", data);
        setResources(data);
      } catch (err) {
        console.error("fetchResources error:", err);
      }
    }

    async function fetchMonsters() {
      try {
        const res = await fetch("http://localhost:8001/api/monsters", { cache: "no-store" });
        if (!res.ok) {
          console.error("Ошибка загрузки монстров", await res.text());
          return;
        }
        const data = await res.json();
        console.log("Полученные монстры:", data);
        setMonsters(data);
      } catch (err) {
        console.error("fetchMonsters error:", err);
      }
    }

    fetchResources();
    fetchMonsters();
  }, [instanceId]);

  // Загружаем данные матча – сервер уже возвращает зафиксированную карту (fullCells)
  useEffect(() => {
    async function fetchMatchData() {
      try {
        const res = await fetch(`http://localhost:8001/game/match?instance_id=${instanceId}`, { cache: "no-store" });
        if (!res.ok) {
          console.error("Ошибка загрузки данных матча", await res.text());
          router.replace("/mode");
          return;
        }
        const data = await res.json();
        console.log("Полученные данные матча:", data);
        console.log("Полученные игроки:", data.players);
        if (data.instance_id !== instanceId) {
          console.log("Актуальный instance_id отличается, обновляем URL:", data.instance_id);
          router.replace(`/game?instance_id=${data.instance_id}`);
          return;
        }
        // Предполагаем, что data.map уже является массивом клеток (с полями cell_id, x, y, tileCode, resource, monster, isPortal)
        const players = (data.players || []).map((p: any) => ({
          ...p,
          position: { x: p.pos_x, y: p.pos_y },
        }));
        dispatch({
          type: "SET_MATCH_DATA",
          payload: {
            instanceId: data.instance_id,
            mode: data.mode,
            grid: data.map, // Используем зафиксированную карту с сервера
            mapWidth: data.map_width,
            mapHeight: data.map_height,
            players,
            activePlayer: data.active_player,
            turnNumber: data.turn_number,
          },
        });
      } catch (err) {
        console.error("fetchMatchData error:", err);
      }
    }
    // Загружаем данные матча только когда ресурсы и монстры уже получены
    if (resources.length > 0 && monsters.length > 0) {
      fetchMatchData();
    }
  }, [instanceId, resources.length, monsters.length, router]);

  // Подключаем WebSocket для обновлений матча
  useGameSocket((data) => {
    console.log("Получено сообщение по WebSocket:", data);
    if (data.payload && data.payload.instanceId && data.payload.instanceId !== state.instanceId) {
      console.log("Сообщение не для текущего матча, игнорируем:", data.payload.instanceId);
      return;
    }
    if (data.type === "MATCH_UPDATE") {
      dispatch({
        type: "SET_MATCH_DATA",
        payload: {
          instanceId: data.payload.instanceId,
          mode: data.payload.mode,
          grid: data.payload.grid,
          mapWidth: data.payload.mapWidth,
          mapHeight: data.payload.mapHeight,
          players: data.payload.players,
          activePlayer: data.payload.active_player,
          turnNumber: data.payload.turn_number,
        },
      });
    } else if (data.type === "MOVE_PLAYER") {
      dispatch({
        type: "MOVE_PLAYER",
        payload: { playerId: data.payload.playerId, newPosition: data.payload.newPosition },
      });
    } else if (data.type === "SET_ACTIVE_PLAYER") {
      const { activePlayer, turnNumber } = data.payload;
      dispatch({ type: "SET_ACTIVE_PLAYER", payload: { activePlayer, turnNumber } });
    }
  }, { instanceId: state.instanceId });

  return (
    <GameContext.Provider value={{ state, dispatch }}>
      {children}
    </GameContext.Provider>
  );
}

export function useGame() {
  const context = useContext(GameContext);
  if (!context) {
    throw new Error("useGame must be used within a GameProvider");
  }
  return context;
}
