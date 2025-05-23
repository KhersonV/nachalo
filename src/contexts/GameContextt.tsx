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
import { useAuth } from "../contexts/AuthContext";
import type { GameState, Action, ResourceType, MonsterType, PlayerState, Cell, Inventory } from "../types/GameTypes";

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

const GameContext = createContext<{ state: GameState; dispatch: Dispatch<Action>; socket: ReturnType<typeof useGameSocket> } | undefined>(undefined);





export function gameReducer(state: GameState, action: Action): GameState {
  switch (action.type) {
    case "SET_MATCH_DATA":
     // console.log("[GameReducer][SET_MATCH_DATA] Получены данные матча:", action.payload);
      const active_user = action.payload.active_user;
     // console.log("[GameReducer] Устанавливаем active_user:", active_user);
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
     // console.log("[GameReducer][MOVE_PLAYER] Перемещение игрока:", action.payload);
      const playerId = action.payload.userId;
      return {
        ...state,
        players: state.players.map((p) =>
          p.user_id === playerId
            ? { ...p, position: action.payload.newPosition }
            : p
        ),
      };

      case "COMBAT_EXCHANGE": {
  const { attacker, target } = action.payload;
  //console.log("[GameReducer][COMBAT_EXCHANGE]", attacker, target);
  return {
    ...state,
    players: state.players.map(p => {
      if (p.user_id === attacker.id) return { ...p, health: attacker.new_hp };
      if (p.user_id === target.id)   return { ...p, health: target.new_hp };
      return p;
    }),
  };
}


 case "BARREL_ARTIFACT": {
  const { updatedPlayer } = action.payload;
  return {
    ...state,
    players: state.players.map(p =>
      p.user_id === updatedPlayer.user_id ? updatedPlayer : p
    ),
  };
}

    case "UPDATE_INVENTORY": {
  const { userId, inventory } = action.payload as {
    userId: number;
    inventory: Inventory;
  };
  // обновляем только у нужного игрока
  return {
    ...state,
    players: state.players.map(p =>
      p.user_id === userId ? { ...p, inventory } : p
    ),
  };
}

   case "SET_ACTIVE_USER": {
 // console.log("[GameReducer][SET_ACTIVE_USER] Новый активный игрок:", action.payload.active_user, "энергия:", action.payload.energy);
  const { active_user, turnNumber, energy } = action.payload;
  return {
    ...state,
    active_user,
    turnNumber,
    // обновляем энергию только у того, чей ход закончился
    players: state.players.map(p =>
      p.user_id === active_user
        ? { ...p, energy: energy ?? p.energy }
        : p
    ),
  };
}

    case "UPDATE_PLAYER":
    //  console.log("[GameReducer][UPDATE_PLAYER] До обновления. Полученные данные:", action.payload.player);
      const updatedData = action.payload.player;
      const normalizedPlayer: PlayerState = {
        ...updatedData,
        position: { x: updatedData.position?.x, y: updatedData.position?.y },
        user_id: updatedData.user_id,
      };
      if (typeof normalizedPlayer.inventory === "string") {
      //  console.log("[GameReducer][UPDATE_PLAYER] Инвентарь как строка:", normalizedPlayer.inventory);
        try {
          normalizedPlayer.inventory = JSON.parse(normalizedPlayer.inventory);
       //   console.log("[GameReducer][UPDATE_PLAYER] Парсенный инвентарь:", normalizedPlayer.inventory);
        } catch (e) {
        //  console.error("[GameReducer][UPDATE_PLAYER] Ошибка парсинга инвентаря:", e);
          normalizedPlayer.inventory = { resources: {}, artifacts: {} };
        }
      }
     // console.log("[GameReducer][UPDATE_PLAYER] После нормализации:", normalizedPlayer);
      return {
        ...state,
        players: state.players.map((p) =>
          p.user_id === normalizedPlayer.user_id ? normalizedPlayer : p
        ),
      };

    case "UPDATE_CELL": {
  const { updatedCell } = action.payload
  return {
    ...state,
    grid: state.grid.map(cell => {
      // если пришёл cell_id и он совпадает — используем его:
      if (updatedCell.cell_id != null && cell.cell_id === updatedCell.cell_id) {
        return { ...cell, ...updatedCell }
      }
      // иначе, если пришли координаты, матчим по ним:
      if (updatedCell.x != null && updatedCell.y != null &&
          cell.x === updatedCell.x && cell.y === updatedCell.y) {
        return { ...cell, ...updatedCell }
      }
      return cell
    })
  }
}

case "PLAYER_DEFEATED": {
 // console.log("[GameReducer] PLAYER_DEFEATED, удаляем игрока:", action.payload.userId);
  return {
    ...state,
    players: state.players.filter(p => p.user_id !== action.payload.userId),
  };
}

case "TURN_PASSED":
      return {
        ...state,
        active_user: action.payload.active_user,
      };

    case "RESET_STATE":
     // console.log("[GameReducer][RESET_STATE] Сброс состояния");
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
  const { user } = useAuth();
  const [state, dispatch] = useReducer(gameReducer, { ...initialState, instanceId });
  const [resources, setResources] = useState<ResourceType[]>([]);
  const [monsters, setMonsters] = useState<MonsterType[]>([]);
  const prevInstanceIdRef = useRef<string>("");

  useEffect(() => {
    //console.log("[GameProvider] instanceId изменился на:", instanceId);
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
       // console.log("[GameProvider] Полученные ресурсы:", data);
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
       // console.log("[GameProvider] Полученные монстры:", data);
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
          
          router.replace(`/game?instance_id=${data.instance_id}`);
          return;
        }
        // Если сервер уже возвращает поле position, используем его без лишних преобразований
        const players = (data.players || []).map((p: any) => {
          return {
            ...p,
            user_id: p.user_id || p.player_id,
            position: { x: p.position.x, y: p.position.y },
          };
        });
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








 const socket = useGameSocket(
  (data) => {
    // Игнорируем сообщения не для текущего матча
    if (data.payload?.instanceId && data.payload.instanceId !== state.instanceId) {
      console.log("[GameProvider] Сообщение не для текущего матча, игнорируем:", data.payload.instanceId);
      return;
    }

    switch (data.type) {
     case "MATCH_UPDATE": {
  const { instanceId: msgMatch, mode, grid, mapWidth, mapHeight, players, active_player, turn_number } = data.payload;
  if (msgMatch !== state.instanceId) break;
  dispatch({
    type: "SET_MATCH_DATA",
    payload: {
      instanceId: state.instanceId,
      mode,
      grid,
      mapWidth,
      mapHeight,
      players,
      active_user: active_player,
      turnNumber: turn_number,
    },
  });
  break;
}


     case "MOVE_PLAYER": {
  // 1. Деструктурируем payload
  const { instanceId: msgMatch, playerId, userId, newPosition } = data.payload;
  // 2. Фильтруем не-наш матч
  if (msgMatch !== state.instanceId) break;
  // 3. Диспатчим, передавая instanceId из state (или msgMatch — они же равны)
  dispatch({
    type: "MOVE_PLAYER",
    payload: {
      instanceId: state.instanceId,
      userId: playerId ?? userId,
      newPosition,
    },
  });
  break;
}

      // 2) RESOURCE_COLLECTED
      case "RESOURCE_COLLECTED": {
        const { instanceId: msgMatch, updatedCell, updatedPlayer } = data.payload;
        if (msgMatch !== state.instanceId) break;
        dispatch({
          type: "UPDATE_CELL",
          payload: {
            instanceId: state.instanceId,
            updatedCell,
          },
        });
        if (updatedPlayer) {
          dispatch({
            type: "UPDATE_PLAYER",
            payload: {
              instanceId: state.instanceId,
              player: updatedPlayer,
            },
          });
        }
        break;
      }
case "SET_ACTIVE_USER": {
  const { instanceId: msgMatch, active_user, turnNumber, energy } = data.payload;
  if (msgMatch !== state.instanceId) break;
  console.log("[GameProvider] SET_ACTIVE_USER:", data.payload);
  dispatch({
    type: "SET_ACTIVE_USER",
    payload: { instanceId: state.instanceId, active_user, turnNumber, energy },
  });
  break;
}

case "UPDATE_PLAYER": {
  const { instanceId: msgMatch, player } = data.payload;
  if (msgMatch !== state.instanceId) break;
  dispatch({
    type: "UPDATE_PLAYER",
    payload: { instanceId: state.instanceId, player },
  });
  break;
}

      
     case "COMBAT_EXCHANGE": {
  // приводим payload к нужному типу
  const { instanceId, attacker, target } = data.payload as {
    instanceId: string;
    attacker: { id: number; new_hp: number; damage: number };
    target:   { id: number; new_hp: number; damage: number };
  };

  dispatch({
    type: "COMBAT_EXCHANGE",
    payload: {
      instanceId,
      attacker: { id: attacker.id, new_hp: attacker.new_hp },
      target:   { id: target.id,   new_hp: target.new_hp },
    },
  });
  break;
}

case "MONSTER_HIT": {
  const { instanceId, monsterInstanceId, newHP, x, y } = data.payload as {
    instanceId: string;
    monsterInstanceId: number;
    newHP: number;
    x: number;
    y: number;
  };

  // Обновляем клетку в grid, меняя здоровье монстра
  dispatch({
    type: "UPDATE_CELL",
    payload: {
      instanceId,
      updatedCell: {
        ...state.grid.find(cell => cell.x === x && cell.y === y)!,
        monster: state.grid
          .find(cell => cell.x === x && cell.y === y)!
          .monster && {
            ...state.grid.find(cell => cell.x === x && cell.y === y)!.monster!,
            health: newHP,
          },
      },
    },
  });
  break;
}



    case "BARREL_DAMAGE": {
      // payload: { userId, amount, hp }
      const {instanceId, userId, amount, hp } = data.payload as {
        instanceId: string;
        userId: number;
        amount: number;
        hp: number;
      };
      // обновляем игрока
      dispatch({
        type: "UPDATE_PLAYER",
        payload: {
          instanceId,
          player: {
            ...state.players.find(p => p.user_id === userId)!,
            health: hp
          }
        }
      });
      break;
    }

      // 5) BARREL_RESOURCE / BARREL_ARTIFACT
      case "BARREL_RESOURCE":
      case "BARREL_ARTIFACT": {
        const { instanceId: msgMatch, updatedCell, updatedPlayer } = data.payload;
        if (msgMatch !== state.instanceId) break;
        dispatch({
          type: "UPDATE_CELL",
          payload: {
            instanceId: state.instanceId,
            updatedCell,
          },
        });
        dispatch({
          type: "UPDATE_PLAYER",
          payload: {
            instanceId: state.instanceId,
            player: updatedPlayer,
          },
        });
        break;
      }


   // 3) UPDATE_INVENTORY
      case "UPDATE_INVENTORY": {
        const { instanceId: msgMatch, userId, inventory } = data.payload;
        if (msgMatch !== state.instanceId) break;
        dispatch({
          type: "UPDATE_INVENTORY",
          payload: {
            instanceId: state.instanceId,
            userId,
            inventory,
          },
        });
        break;
      }



  // 4) UPDATE_CELL
      case "UPDATE_CELL": {
        const raw = data.payload as any;
        const msgMatch = raw.instanceId ?? raw.payload?.instanceId;
        if (msgMatch !== state.instanceId) break;
        const updatedCell = raw.updatedCell ?? raw;
        dispatch({
          type: "UPDATE_CELL",
          payload: {
            instanceId: state.instanceId,
            updatedCell: updatedCell as Cell,
          },
        });
        break;
      }

case "PLAYER_DEFEATED": {
  const { instanceId: msgMatch, userId } = data.payload as {
    instanceId: string;
    userId: number;
  };
  // Фильтруем чужие матчи
  if (msgMatch !== state.instanceId) break;
  // Для себя — редирект
  if (user?.id != null && userId === user.id) {
    router.push("/mode");
  }
  // Удаляем игрока из списка
  dispatch({
    type: "PLAYER_DEFEATED",
    payload: { userId },
  });
  break;
}

  case "TURN_PASSED": {
  // 1) Деструктурируем из payload и сразу переименовываем
  const { instanceId: msgMatch, active_user: active_user } = data.payload as {
    instanceId: string;
    active_user: number;
  };
  // 2) Фильтруем «чужие» матчи
  if (msgMatch !== state.instanceId) break;
  // 3) Логируем и диспатчим только для нашего матча
  console.log("[GameProvider] TURN_PASSED:", data.payload);
  dispatch({
    type: "TURN_PASSED",
    payload: { active_user },
  });
  break;
}

      default:
        // можно логировать непредвидённые типы
        console.warn("[GameProvider] Неизвестный тип WS-сообщения:", data.type);
    }
  },
  { instanceId: state.instanceId }
);


 return (
  <GameContext.Provider value={{ state, dispatch, socket }}>
    {children}
  </GameContext.Provider>
);

}

export function useGame() {
  const context = useContext(GameContext);
  if (!context) throw new Error("useGame must be used within a GameProvider");
  return context; // теперь здесь есть state, dispatch и socket
}
