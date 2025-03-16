//==================================
// src/contexts/GameContextt.tsx
//==================================

"use client";

import React, {
  createContext,
  useContext,
  useReducer,
  useEffect,
  Dispatch,
  useState,
  useRef,
} from "react";
import { useRouter } from "next/navigation";
import { useGameSocket } from "../hooks/useGameSocket";
import type { GameState, Action, ResourceType, MonsterType, Cell, PlayerState } from "../types/GameTypes";

// Начальное состояние
const initialState: GameState = {
  instanceId: "",
  mode: "",
  grid: [],
  mapWidth: 0,
  mapHeight: 0,
  players: [],
  active_user: 0,
  turnNumber: 1,
};

const GameContext = createContext<{ state: GameState; dispatch: Dispatch<Action> } | undefined>(undefined);

export function gameReducer(state: GameState, action: Action): GameState {
  switch (action.type) {
    case "SET_MATCH_DATA":
      console.log("[GameReducer][SET_MATCH_DATA] Получены данные матча:", action.payload);
      const active_user = action.payload.active_user;
      console.log("[GameReducer] Устанавливаем active_user:", active_user);
      return {
        ...state,
        instanceId: action.payload.instanceId,
        mode: action.payload.mode,
        grid: action.payload.grid,
        mapWidth: action.payload.mapWidth,
        mapHeight: action.payload.mapHeight,
        players: action.payload.players,
        active_user: active_user,
        turnNumber: action.payload.turnNumber || state.turnNumber,
      };

    case "MOVE_PLAYER":
      console.log("[GameReducer][MOVE_PLAYER] Перемещение игрока:", action.payload);
      const playerId = action.payload.userId;
      return {
        ...state,
        players: state.players.map((p) =>
          p.user_id === playerId
            ? { ...p, position: action.payload.newPosition }
            : p
        ),
      };

    case "SET_ACTIVE_USER":
      console.log("[GameReducer][SET_ACTIVE_USER] Новый активный игрок:", action.payload.active_user);
      return {
        ...state,
        active_user: action.payload.active_user,
        turnNumber: action.payload.turnNumber,
      };

    case "UPDATE_PLAYER":
      console.log("[GameReducer][UPDATE_PLAYER] До обновления. Полученные данные:", action.payload.player);
      const updatedData = action.payload.player;
      const normalizedPlayer: PlayerState = {
        ...updatedData,
        position: { x: updatedData.position?.x, y: updatedData.position?.y },
        user_id: updatedData.user_id,
      };
      if (typeof normalizedPlayer.inventory === "string") {
        console.log("[GameReducer][UPDATE_PLAYER] Инвентарь как строка:", normalizedPlayer.inventory);
        try {
          normalizedPlayer.inventory = JSON.parse(normalizedPlayer.inventory);
          console.log("[GameReducer][UPDATE_PLAYER] Парсенный инвентарь:", normalizedPlayer.inventory);
        } catch (e) {
          console.error("[GameReducer][UPDATE_PLAYER] Ошибка парсинга инвентаря:", e);
          normalizedPlayer.inventory = { resources: {}, artifacts: {} };
        }
      }
      console.log("[GameReducer][UPDATE_PLAYER] После нормализации:", normalizedPlayer);
      return {
        ...state,
        players: state.players.map((p) =>
          p.user_id === normalizedPlayer.user_id ? normalizedPlayer : p
        ),
      };

    case "UPDATE_CELL":
      // Обновляем только ту клетку, id которой совпадает с updatedCell.id
      const { updatedCell } = action.payload;
      return {
        ...state,
        grid: state.grid.map((cell) =>
          cell.cell_id === updatedCell.cell_id ? { ...cell, ...updatedCell } : cell
        ),
      };

    case "RESET_STATE":
      console.log("[GameReducer][RESET_STATE] Сброс состояния");
      return { ...initialState };

    default:
      return state;
  }
}

type GameProviderProps = {
  instanceId: string;
  children: React.ReactNode;
};

export function GameProvider({ instanceId, children }: GameProviderProps) {
  const router = useRouter();
  const [state, dispatch] = useReducer(gameReducer, { ...initialState, instanceId });
  const [resources, setResources] = useState<ResourceType[]>([]);
  const [monsters, setMonsters] = useState<MonsterType[]>([]);
  const prevInstanceIdRef = useRef<string>("");

  useEffect(() => {
    console.log("[GameProvider] instanceId изменился на:", instanceId);
    // Сброс состояния, если instanceId изменился
    if (prevInstanceIdRef.current && prevInstanceIdRef.current !== instanceId) {
      dispatch({ type: "RESET_STATE" });
    }
    prevInstanceIdRef.current = instanceId;

    async function fetchResources() {
      try {
        const res = await fetch("http://localhost:8001/api/resources", { cache: "no-store" });
        if (!res.ok) {
          console.error("[GameProvider] Ошибка загрузки ресурсов", await res.text());
          return;
        }
        const data = await res.json();
        console.log("[GameProvider] Полученные ресурсы:", data);
        setResources(data);
      } catch (err) {
        console.error("[GameProvider] fetchResources error:", err);
      }
    }

    async function fetchMonsters() {
      try {
        const res = await fetch("http://localhost:8001/api/monsters", { cache: "no-store" });
        if (!res.ok) {
          console.error("[GameProvider] Ошибка загрузки монстров", await res.text());
          return;
        }
        const data = await res.json();
        console.log("[GameProvider] Полученные монстры:", data);
        setMonsters(data);
      } catch (err) {
        console.error("[GameProvider] fetchMonsters error:", err);
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
          console.error("[GameProvider] Ошибка загрузки данных матча", await res.text());
          router.replace("/mode");
          return;
        }
        const data = await res.json();
        console.log("[GameProvider] Полученные данные матча:", data);
        if (data.instance_id !== instanceId) {
          console.log("[GameProvider] Актуальный instance_id отличается, обновляем URL:", data.instance_id);
          router.replace(`/game?instance_id=${data.instance_id}`);
          return;
        }
        // Если сервер уже возвращает поле position, используем его без лишних преобразований
        const players = (data.players || []).map((p: any) => {
          console.log("[GameProvider] Original player data:", p);
          return {
            ...p,
            user_id: p.user_id || p.player_id,
            position: { x: p.position.x, y: p.position.y },
          };
        });

        console.log("[GameProvider] Игроки после конвертации:", players);
        dispatch({
          type: "SET_MATCH_DATA",
          payload: {
            instanceId: data.instance_id,
            mode: data.mode,
            grid: data.map,
            mapWidth: data.map_width,
            mapHeight: data.map_height,
            players,
            active_user: data.active_user,
            turnNumber: data.turn_number,
          },
        });
      } catch (err) {
        console.error("[GameProvider] fetchMatchData error:", err);
      }
    }
    if (resources.length > 0 && monsters.length > 0) {
      fetchMatchData();
    }
  }, [instanceId, resources.length, monsters.length, router]);

  useGameSocket(
    (data) => {
      console.log("[GameProvider] Получено сообщение по WebSocket:", data);
      if (data.payload && data.payload.instanceId && data.payload.instanceId !== state.instanceId) {
        console.log("[GameProvider] Сообщение не для текущего матча, игнорируем:", data.payload.instanceId);
        return;
      }
      if (data.type === "MATCH_UPDATE") {
        // Предполагаем, что сервер отправляет обновленный grid в data.payload.grid
        dispatch({
          type: "SET_MATCH_DATA",
          payload: {
            instanceId: data.payload.instanceId,
            mode: data.payload.mode,
            grid: data.payload.grid,
            mapWidth: data.payload.mapWidth,
            mapHeight: data.payload.mapHeight,
            players: data.payload.players,
            active_user: data.payload.active_player, // сервер может использовать active_player
            turnNumber: data.payload.turn_number,
          },
        });
      } else if (data.type === "MOVE_PLAYER") {
        dispatch({
          type: "MOVE_PLAYER",
          payload: {
            userId: data.payload.playerId || data.payload.userId,
            newPosition: data.payload.newPosition,
          },
        });
      }

      else if (data.type === "RESOURCE_COLLECTED") {
        // Обновляем клетку в grid
        const { updatedCell } = data.payload;
        dispatch({
          type: "UPDATE_CELL",
          payload: { updatedCell },
        });
      }
      else if (data.type === "SET_ACTIVE_USER") {
        console.log("[GameProvider] SET_ACTIVE_USER:", data.payload);
        const { active_user, turnNumber } = data.payload;
        dispatch({ type: "SET_ACTIVE_USER", payload: { active_user, turnNumber } });
      }
    },
    { instanceId: state.instanceId }
  );

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
