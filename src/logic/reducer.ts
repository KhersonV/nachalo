//reduser.ts

import { GameState   } from "./types";
import { Action } from "./actions";
import { resources } from "./allData";

import { generateBattlefield } from './generateBattlefield';


export function gameReducer(state: GameState, action: Action): GameState {
  switch (action.type) {
    case "INITIALIZE_GAME":
      return {
        ...state,
        mode: action.payload.mode,
        instanceId: action.payload.instanceId,
        players: action.payload.players,
      };

    case "SET_GRID":
      return {
        ...state,
        grid: action.payload.grid,
      };

    case "MOVE_PLAYER": {
      const { playerId, newPosition } = action.payload;
      const updatedPlayers = state.players.map((player) =>
        player.id === playerId
          ? { ...player, position: newPosition, energy: player.energy - 1 }
          : player
      );
      return { ...state, players: updatedPlayers };
    }

    case "ATTACK": {
      const { attackerId, targetId, damage, targetType, cellId } = action.payload;

      if (targetType === "player") {
        const updatedPlayers = state.players.map((player) =>
          player.id === targetId
            ? { ...player, health: Math.max(player.health - damage, 0) }
            : player
        );
        return { ...state, players: updatedPlayers };
      } else if (targetType === "monster" && cellId !== undefined) {
        const updatedGrid = state.grid.map((cell) => {
          if (cell.id === cellId && cell.monster) {
            const newHp = Math.max(cell.monster.hp - damage, 0);
            return newHp > 0
              ? { ...cell, monster: { ...cell.monster, hp: newHp } }
              : { ...cell, monster: undefined };
          }
          return cell;
        });
        return { ...state, grid: updatedGrid };
      }
      return state;
    }

    case "COLLECT_RESOURCE": {
      const { playerId, resourceType, cellId } = action.payload;
      if (!resources[resourceType]) return state;

      const updatedPlayers = state.players.map((player) =>
        player.id === playerId
          ? {
              ...player,
              inventory: {
                ...player.inventory,
                [resourceType]: {
                  count: (player.inventory[resourceType]?.count || 0) + 1,
                  description: resources[resourceType].description,
                  image: `/main_resources/${resourceType}.webp`,
                },
              },
            }
          : player
      );

      const updatedGrid = state.grid.map((cell) =>
        cell.id === cellId ? { ...cell, resource: null } : cell
      );

      return { ...state, players: updatedPlayers, grid: updatedGrid };
    }

    case "USE_ITEM": {
      const { playerId, itemType } = action.payload;

      const updatedPlayers = state.players.map((player) => {
        if (player.id === playerId && player.inventory[itemType]?.count > 0) {
          const resourceEffect = resources[itemType]?.effect || 0;

          return {
            ...player,
            inventory: {
              ...player.inventory,
              [itemType]: {
                ...player.inventory[itemType],
                count: player.inventory[itemType].count - 1,
              },
            },
            // Применяем эффект ресурса
            ...(itemType === "food"
              ? { health: Math.min(player.maxHealth, player.health + resourceEffect) }
              : {}),
            ...(itemType === "water"
              ? { energy: Math.min(player.maxEnergy, player.energy + resourceEffect) }
              : {}),
          };
        }
        return player;
      });

      return { ...state, players: updatedPlayers };
    }

    case "PASS_TURN": {
      const nextIndex = (state.currentPlayerIndex + 1) % state.players.length;
      const isEndOfTurn = nextIndex === 0;

      let updatedState = {
        ...state,
        currentPlayerIndex: nextIndex,
      };

      if (isEndOfTurn) {
        updatedState = {
          ...updatedState,
          turnCycle: state.turnCycle + 1,
          monstersHaveAttacked: false, // Сброс флага атаки монстров
        };
      }

      return updatedState;
    }

    case "UPDATE_MONSTER_ATTACK_TURN": {
      const { monsterId, turnCycle } = action.payload;

      const updatedGrid = state.grid.map((cell) =>
        cell.monster?.id === monsterId
          ? { ...cell, monster: { ...cell.monster, lastTurnAttacked: turnCycle } }
          : cell
      );

      return { ...state, grid: updatedGrid };
    }

    case "UPDATE_PLAYER_STATS": {
      const { playerId, stats } = action.payload;
      return {
        ...state,
        players: state.players.map((player) =>
          player.id === playerId ? { ...player, ...stats } : player
        ),
      };
    }
    
    case "REMOVE_PLAYER": {
      const { playerId } = action.payload;
      return {
        ...state,
        players: state.players.filter((player) => player.id !== playerId),
      };
    }

    case 'START_BATTLE': {
      const { attacker, defender } = action.payload; 
      // attacker и defender - это PlayerState | MonsterState

      const battlefield = generateBattlefield(11, 15, attacker, defender);

      // Можно установить флаг inBattle
      return {
        ...state,
        inBattle: true,
        // Если вы хотите где-то хранить поле боя, можно добавить его в состояние:
        // battlefield: battlefield
      };
    }
    
    
    
    case "SET_MONSTERS_HAVE_ATTACKED":
      return { ...state, monstersHaveAttacked: action.payload.monstersHaveAttacked };

    case "TOGGLE_INVENTORY":
      return { ...state, inventoryOpen: !state.inventoryOpen };

    default:
      return state;
  }
}
