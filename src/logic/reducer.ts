// src/logic/reducer.ts

import { GameState, PlayerState, Cell } from "./types";
import { Action, Reward } from "./actions";
import { resources } from "../logic//ResourceData";

export function gameReducer(state: GameState, action: Action): GameState {
  switch (action.type) {
    case 'INITIALIZE_GAME':
      return {
        ...state,
        mode: action.payload.mode,
        instanceId: action.payload.instanceId,
        players: action.payload.players,
        // Инициализируйте другие поля по необходимости
      };

    case 'SET_INSTANCE_ID':
      return {
        ...state,
        instanceId: action.payload.instanceId,
      };

    case 'MOVE_PLAYER': {
      const { playerId, newPosition } = action.payload;
      const updatedPlayers = state.players.map(player =>
        player.id === playerId
          ? { ...player, position: newPosition, energy: player.energy - 1 }
          : player
      );
      return { ...state, players: updatedPlayers };
    }

    case 'ATTACK': {
      const { attackerId, targetId, damage } = action.payload;
      const updatedPlayers = state.players.map(player => {
        if (player.id === targetId) {
          const newHealth = Math.max(player.health - damage, 0);
          return { ...player, health: newHealth };
        }
        return player;
      });
      return { ...state, players: updatedPlayers };
    }

    case 'COLLECT_RESOURCE': {
      const { playerId, resourceType } = action.payload;
      const resource = resources[resourceType];
      if (!resource) return state;

      const updatedPlayers = state.players.map(player => {
        if (player.id === playerId) {
          const currentCount = player.inventory[resourceType]?.count || 0;
          return {
            ...player,
            inventory: {
              ...player.inventory,
              [resourceType]: {
                ...player.inventory[resourceType],
                count: currentCount + 1,
              },
            },
            // Примените эффекты ресурса, если необходимо
            ...(resource.effect && resourceType === 'food' ? { health: Math.min(player.maxHealth, player.health + resource.effect) } : {}),
            ...(resource.effect && resourceType === 'water' ? { energy: Math.min(player.maxEnergy, player.energy + resource.effect) } : {}),
            // Добавьте другие эффекты по необходимости
          };
        }
        return player;
      });

      // Удалите ресурс с клетки, если это необходимо
      const updatedGrid = state.grid?.map(cell => {
        if (cell.resource && cell.resource.type === resourceType) {
          return { ...cell, resource: null };
        }
        return cell;
      });

      return { ...state, players: updatedPlayers, grid: updatedGrid || state.grid };
    }

    case 'USE_ITEM': {
      const { playerId, itemType } = action.payload;
      const resource = resources[itemType];
      if (!resource) return state;

      const updatedPlayers = state.players.map(player => {
        if (player.id === playerId && player.inventory[itemType]?.count > 0) {
          const newCount = player.inventory[itemType].count - 1;
          const updatedInventory = { ...player.inventory };
          if (newCount <= 0) {
            delete updatedInventory[itemType];
          } else {
            updatedInventory[itemType].count = newCount;
          }

          // Примените эффекты предмета
          let updatedPlayer = { ...player, inventory: updatedInventory };
          switch (itemType) {
            case 'food':
              updatedPlayer.health = Math.min(player.maxHealth, player.health + resource.effect);
              break;
            case 'water':
              updatedPlayer.energy = Math.min(player.maxEnergy, player.energy + resource.effect);
              break;
            // Добавьте другие случаи по необходимости
            default:
              break;
          }

          return updatedPlayer;
        }
        return player;
      });

      return { ...state, players: updatedPlayers };
    }

    case 'PASS_TURN': {
      const nextIndex = (state.currentPlayerIndex + 1) % state.players.length;
      const isEndOfTurn = nextIndex === 0;
      let updatedState = { ...state, currentPlayerIndex: nextIndex };
      if (isEndOfTurn) {
        updatedState.turnCycle = state.turnCycle + 1;
        // Вызовите функции для атаки монстров или другие действия конца хода
      }
      return updatedState;
    }

    case 'PICK_ARTIFACT': {
      const { playerId } = action.payload;
      return { ...state, artifactOwner: playerId };
    }

    case 'LOSE_ARTIFACT': {
      const { playerId } = action.payload;
      if (state.artifactOwner !== playerId) return state;
      return { ...state, artifactOwner: null };
    }

    case 'ADD_ITEM': {
      const { playerId, itemType, description, image, bonus } = action.payload;
      const updatedPlayers = state.players.map(player => {
        if (player.id === playerId) {
          const currentCount = player.inventory[itemType]?.count || 0;
          return {
            ...player,
            inventory: {
              ...player.inventory,
              [itemType]: {
                count: currentCount + 1,
                description,
                image,
                ...(bonus && { bonus }),
              },
            },
          };
        }
        return player;
      });

      return { ...state, players: updatedPlayers };
    }

    case 'REMOVE_RESOURCE': {
      const { cellId } = action.payload;
      const updatedGrid = state.grid?.map(cell =>
        cell.id === cellId ? { ...cell, resource: null } : cell
      );

      return { ...state, grid: updatedGrid || state.grid };
    }

    case 'ADD_MONSTER': {
      const { cellId, monster } = action.payload;
      const updatedGrid = state.grid?.map(cell =>
        cell.id === cellId ? { ...cell, monster } : cell
      );

      return { ...state, grid: updatedGrid || state.grid };
    }

    case 'TRY_EXIT_PORTAL': {
      const { playerId } = action.payload;
      const player = state.players.find(p => p.id === playerId);
      if (!player) return state;

      if (state.artifactOwner === playerId) {
        console.log(`Игрок ${player.name} успешно покидает портал с артефактом!`);
        // Добавьте логику завершения игры или перехода на следующий уровень
        // Например:
        // dispatch({ type: 'FINALIZE_INSTANCE', payload: { instanceId: state.instanceId, players: state.players } });
        return state; // Обновите состояние по необходимости
      } else {
        console.log("Игрок не может выйти через портал без артефакта.");
        return state;
      }
    }

    case 'PLAYER_DIED': {
      const { playerId } = action.payload;
      const updatedPlayers = state.players.map(player =>
        player.id === playerId
          ? { ...player, health: 0 } // Можно добавить логику для "мертвого" игрока
          : player
      );

      // Опционально: завершение игры, если игрок мертв
      // Например:
      // const isGameOver = updatedPlayers.some(p => p.health <= 0);
      // if (isGameOver) { dispatch({ type: 'FINALIZE_INSTANCE', payload: { instanceId: state.instanceId, players: updatedPlayers } }); }

      return { ...state, players: updatedPlayers };
    }

    case 'FINALIZE_INSTANCE': {
      const { instanceId, players } = action.payload;
      console.log(`Инстанс ${instanceId} завершён. Начисляем награды игрокам.`);
      players.forEach(player => {
        console.log(`Игрок ${player.name} получает опыт и валюту.`);
        // Добавьте действия для начисления наград
        // Например:
        // dispatch({ type: 'AWARD_REWARD', payload: { playerId: player.id, reward: { experience: 100, currency: 50 } } });
      });
      // Логика завершения игры, например, перенаправление пользователя
      return state; // Обновите состояние по необходимости
    }

    case 'SET_GRID':
      return {
        ...state,
        grid: action.payload.grid,
      };

    case 'AWARD_REWARD': {
      const { playerId, reward } = action.payload;
      const updatedPlayers = state.players.map(player => {
        if (player.id === playerId) {
          return {
            ...player,
            expirience: player.expirience + reward.experience,
            // Предполагается, что у PlayerState есть поле для валюты
            // currency: (player.currency || 0) + reward.currency,
          };
        }
        return player;
      });

      return { ...state, players: updatedPlayers };
    }

    case 'TOGGLE_INVENTORY':
      return {
        ...state,
        inventoryOpen: !state.inventoryOpen,
      };

    default:
      return state;
  }
}
