// src/contexts/GameContextt.tsx
"use client";

import React, { createContext, useContext, useReducer, useEffect, Dispatch, useState } from "react";

// Типы для бонусов, инвентаря и прочего
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

// Тип для игрока
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

// Тип для монстра (упрощённый вариант, ожидающий, что API вернёт поле image как строку)
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
  image: string; // Одна картинка для каждого монстра
  created_at: string;
};

// Тип для ресурса
export type ResourceType = {
  id: number;
  type: string;
  description: string;
  effect: Record<string, number>;
  image: string;
};

// Тип для клетки карты
export type Cell = {
  id: number;
  x: number;
  y: number;
  tileCode: number;
  resource: ResourceType | null;
  monster: MonsterType | null;
  isPortal?: boolean;
};

// Тип для состояния игры
export type GameState = {
  instanceId: string;
  mode: string;
  grid: Cell[];
  mapWidth: number;
  mapHeight: number;
  players: PlayerState[];
  currentPlayerId: number;
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
      };
    }
  | {
      type: "MOVE_PLAYER";
      payload: { playerId: number; newPosition: { x: number; y: number } };
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
};

const GameContext = createContext<{ state: GameState; dispatch: Dispatch<Action> } | undefined>(undefined);

export function gameReducer(state: GameState, action: Action): GameState {
  switch (action.type) {
    case "SET_MATCH_DATA":
      // Если currentPlayerId ещё не установлен и есть хотя бы один игрок, устанавливаем его
      let currentId = state.currentPlayerId;
      if (currentId === 0 && action.payload.players.length > 0) {
        currentId = action.payload.players[0].id;
      }
      return {
        ...state,
        instanceId: action.payload.instanceId,
        mode: action.payload.mode,
        grid: action.payload.grid,
        mapWidth: action.payload.mapWidth,
        mapHeight: action.payload.mapHeight,
        players: action.payload.players,
        currentPlayerId: currentId,
      };
    case "MOVE_PLAYER":
      return {
        ...state,
        players: state.players.map((p) =>
          p.id === action.payload.playerId ? { ...p, position: action.payload.newPosition } : p
        ),
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

// Функция для преобразования карты
function convertMapData(rawMap: number[][], resources: ResourceType[], monsters: MonsterType[]): Cell[] {
  const cells: Cell[] = [];
  let id = 0;
  for (let y = 0; y < rawMap.length; y++) {
    for (let x = 0; x < rawMap[y].length; x++) {
      const tileCode = rawMap[y][x];
      let resource: ResourceType | null = null;
      let monster: MonsterType | null = null;
      let isPortal = false;

      // Если tileCode равен 82 ('R') – клетка с ресурсом
      if (tileCode === 82) {
        if (resources && resources.length > 0) {
          const randomIndex = Math.floor(Math.random() * resources.length);
          resource = resources[randomIndex];
        }
      }
      // Если tileCode равен 77 ('M') – клетка с монстром
      if (tileCode === 77) {
        if (monsters && monsters.length > 0) {
          const randomIndex = Math.floor(Math.random() * monsters.length);
          monster = monsters[randomIndex];
        }
      }
      // Если tileCode равен 112 ('p') – клетка с порталом
      if (tileCode === 112) {
        isPortal = true;
      }

      cells.push({
        id: id++,
        x,
        y,
        tileCode,
        resource,
        monster,
        isPortal,
      });
    }
  }
  return cells;
}

export function GameProvider({ instanceId, children }: GameProviderProps) {
  const [state, dispatch] = useReducer(gameReducer, { ...initialState, instanceId });
  const [resources, setResources] = useState<ResourceType[]>([]);
  const [monsters, setMonsters] = useState<MonsterType[]>([]);

  useEffect(() => {
    console.log("instanceId изменился на:", instanceId);
    dispatch({ type: "RESET_STATE" });

    // Загружаем ресурсы
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

    // Загружаем монстров
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

  useEffect(() => {
    // После того как загрузились ресурсы и монстры, загружаем данные матча
    async function fetchMatchData() {
      try {
        const res = await fetch(`http://localhost:8001/game/match?instance_id=${instanceId}`, {
          cache: "no-store",
        });
        if (!res.ok) {
          console.error("Ошибка загрузки данных матча", await res.text());
          return;
        }
        const data = await res.json();
        console.log("Полученные данные матча:", data);
        console.log("Полученные игроки:", data.players);
        const convertedGrid = convertMapData(data.map, resources, monsters);
        const players: PlayerState[] = data.players || [];
        // Преобразуем каждого игрока, добавляя поле position (из pos_x, pos_y)
        const transformedPlayers = players.map((p: any) => ({
          ...p,
          position: { x: p.pos_x, y: p.pos_y },
        }));
        if (transformedPlayers.length === 0) {
          console.warn("Массив игроков пуст!");
        }
        dispatch({
          type: "SET_MATCH_DATA",
          payload: {
            instanceId: data.instance_id,
            mode: data.mode,
            grid: convertedGrid,
            mapWidth: data.map_width,
            mapHeight: data.map_height,
            players: transformedPlayers,
          },
        });
      } catch (err) {
        console.error("fetchMatchData error:", err);
      }
    }
    if (resources.length > 0 && monsters.length > 0) {
      fetchMatchData();
    }
  }, [instanceId, resources.length, monsters.length]);

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
