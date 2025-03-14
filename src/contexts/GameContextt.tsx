// ==============================
// src/contexts/GameContextt.tsx
// ==============================

"use client";

import React, { createContext, useContext, useReducer, useEffect, Dispatch, useState } from "react";
import { useRouter } from "next/navigation";
import { useGameSocket } from "../hooks/useGameSocket";
import type { GameState, Action, ResourceType, MonsterType, Cell } from "../types/GameTypes";

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

const GameContext = createContext<{ state: GameState; dispatch: Dispatch<Action> } | undefined>(undefined);

export function gameReducer(state: GameState, action: Action): GameState {
  switch (action.type) {
    case "SET_MATCH_DATA":
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

// Если карта уже пришла с сервера в виде объектов (с полем cell_id), преобразуем их в тип Cell.
function convertMapData(rawMap: any, resources: ResourceType[], monsters: MonsterType[]): Cell[] {
  // Если массив не пустой и первый элемент имеет поле cell_id – это уже объект.
  if (Array.isArray(rawMap) && rawMap.length > 0 && rawMap[0].cell_id !== undefined) {
    return rawMap.map((cell: any) => ({
      id: cell.cell_id,
      x: cell.x,
      y: cell.y,
      tileCode: cell.tileCode,
      resource: cell.resource,
      monster: cell.monster,
      isPortal: cell.isPortal,
    }));
  }
  // Если приходит двумерный массив чисел, можно его преобразовать (если требуется).
  const cells: Cell[] = [];
  let cellId = 0;
  for (let y = 0; y < rawMap.length; y++) {
    for (let x = 0; x < rawMap[y].length; x++) {
      cells.push({
        id: cellId++,
        x,
        y,
        tileCode: rawMap[y][x],
        resource: null,
        monster: null,
        isPortal: false,
      });
    }
  }
  return cells;
}

export function GameProvider({ instanceId, children }: GameProviderProps) {
  const router = useRouter();
  const [state, dispatch] = useReducer(gameReducer, { ...initialState, instanceId });
  const [resources, setResources] = useState<ResourceType[]>([]);
  const [monsters, setMonsters] = useState<MonsterType[]>([]);

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
        const convertedGrid = convertMapData(data.map, resources, monsters);
        const players = (data.players || []).map((p: any) => ({
          ...p,
          position: { x: p.pos_x, y: p.pos_y },
        }));
        if (players.length === 0) {
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
            players,
            activePlayer: data.active_player,
            turnNumber: data.turn_number,
          },
        });
      } catch (err) {
        console.error("fetchMatchData error:", err);
      }
    }
    if (resources.length > 0 && monsters.length > 0) {
      fetchMatchData();
    }
  }, [instanceId, resources.length, monsters.length, router]);

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
