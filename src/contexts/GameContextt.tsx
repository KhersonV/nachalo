
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//************************************************* src/components/GameContextt.tsx ********************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************


"use client";

import React, {
    createContext,
    useContext,
    useReducer,
    useEffect,
    Dispatch,
} from "react";


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
    name?: string; // Название предмета (не обязательно)
    count: number;
    image: string;
    description: string;
    bonus?: BonusAttributes;
};

export type Inventory = {
    resources: Record<string, InventoryItem>;
    artifacts: Record<string, InventoryItem>;
};

// Пример типов (вы можете расширять или изменять их согласно своей схеме)
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

    // Параметры для тактического боя
    speed: number;           // На сколько клеток ходит в бою
    maneuverability: number; // Сколько клеток может менять позицию при расстановке
    rangeAttack?: boolean;
    rangeDistance?: number;
    // и другие поля, которые нужны для игры…
};

export type MonsterState = {
    id: number;
    name: string;
    health: number;
    maxHealth: number;
    attack: number;
    defense: number;
    vision: number;
    image:  string; // Для MonsterState, image как Record<string, string>
    lastTurnAttacked?: number;
    type: "aggressive" | "neutral";
    speed: number;          // На сколько клеток ходит в бою
    maneuverability: number;// Сколько клеток может поменять позицию перед боем
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
};

export type Cell = {
    id: number;
    x: number;
    y: number;
    tileCode: number;
    resource: ResourceType | null;
    isPortal?: boolean;
    monster?: MonsterState;
    // можно добавить resource, monster, isPortal и т.д.
};

export type GameState = {
    instanceId: string;
    mode: string;
    grid: Cell[];
    mapWidth: number;
    mapHeight: number;
    players: PlayerState[];
    currentPlayerId: number;
    // Другие поля: inBattle, battleParticipants, inventoryOpen, artifactSelection и т.д.
};

export type Action =
    | {
        type: "SET_MATCH_DATA"; payload: {
            instanceId: string;
            mode: string;
            grid: Cell[];
            mapWidth: number;
            mapHeight: number;
            players: PlayerState[];
        }
    }
    | { type: "MOVE_PLAYER"; payload: { playerId: number; newPosition: { x: number; y: number } } }
// Добавляйте другие экшены по необходимости (например, для боя и т.д.)

// Начальное состояние – пустое, затем заполняется при загрузке данных с сервера
const initialState: GameState = {
    instanceId: "",
    mode: "",
    grid: [],
    mapWidth: 0,
    mapHeight: 0,
    players: [],
    currentPlayerId: 0,
};

const GameContext = createContext<{
    state: GameState;
    dispatch: Dispatch<Action>;
} | undefined>(undefined);

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
            };
        case "MOVE_PLAYER":
            return {
                ...state,
                players: state.players.map((p) =>
                    p.id === action.payload.playerId
                        ? { ...p, position: action.payload.newPosition }
                        : p
                ),
            };
        // Добавляйте другие действия здесь...
        default:
            return state;
    }
}

type GameProviderProps = {
    instanceId: string;
    children: React.ReactNode;
};

function convertMapData(rawMap: number[][]): Cell[] {
    const cells: Cell[] = [];
    let id = 0;
    for (let y = 0; y < rawMap.length; y++) {
      for (let x = 0; x < rawMap[y].length; x++) {
        cells.push({
          id: id++,
          x,
          y,
          tileCode: rawMap[y][x],
          resource: null, // Здесь можно добавить логику для определения ресурса
          isPortal: false, // По умолчанию или определённая логика
        });
      }
    }
    return cells;
  }
  

export function GameProvider({ instanceId, children }: GameProviderProps) {
    const [state, dispatch] = useReducer(gameReducer, { ...initialState, instanceId });

    useEffect(() => {
        // Функция для загрузки данных матча (например, с API Game-сервиса)
        async function fetchMatchData() {
            try {
                const res = await fetch(`http://localhost:8001/game/match?instance_id=${instanceId}`);
                if (!res.ok) {
                    console.error("Ошибка загрузки данных матча", await res.text());
                    return;
                }
                const data = await res.json();
                 // Предположим, data.map приходит как двумерный массив чисел
                const convertedGrid = convertMapData(data.map);
                // Приведём данные к нужной форме.
                // Например, data.map - это двумерный массив чисел, data.map_width, data.map_height и т.д.
                // Предположим, что данные игроков тоже приходят с API (либо можно сделать дополнительный запрос).
                const players: PlayerState[] = data.players || []; // если players передаются в match
                // Если список игроков не передаётся, можно задать дефолтные стартовые позиции:
                if (players.length === 0) {
                    players.push({
                      id: 1,
                      name: "Player 1",
                      health: 100,
                      maxHealth: 100,
                      attack: 10,
                      defense: 5,
                      vision: 5,
                      image: "/player-1.webp",
                      colorClass: "red-player",
                      lastTurnAttacked: undefined,
                      position: { x: 1, y: 1 },
                      energy: 100,
                      maxEnergy: 100,
                      level: 1,
                      experience: 0,
                      maxExperience: 500,
                      visionRange: 5,
                      inventory: { resources: {}, artifacts: {} },
                      speed: 3,
                      maneuverability: 2,
                      rangeAttack: false,
                      rangeDistance: 0,
                    });
                    players.push({
                      id: 2,
                      name: "Player 2",
                      health: 100,
                      maxHealth: 100,
                      attack: 10,
                      defense: 5,
                      vision: 5,
                      image: "/player-2.webp",
                      colorClass: "blue-player",
                      lastTurnAttacked: undefined,
                      position: { x: 2, y: 1 },
                      energy: 100,
                      maxEnergy: 100,
                      level: 1,
                      experience: 0,
                      maxExperience: 500,
                      visionRange: 5,
                      inventory: { resources: {}, artifacts: {} },
                      speed: 3,
                      maneuverability: 2,
                      rangeAttack: false,
                      rangeDistance: 0,
                    });
                  }
                  
                dispatch({
                    type: "SET_MATCH_DATA",
                    payload: {
                        instanceId: data.instance_id,
                        mode: data.mode,
                        grid: convertedGrid, // передаем преобразованные данные
                        mapWidth: data.map_width,
                        mapHeight: data.map_height,
                        players,
                    },
                });
            } catch (err) {
                console.error("fetchMatchData error:", err);
            }
        }
        fetchMatchData();
    }, [instanceId]);

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
