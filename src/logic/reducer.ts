// src/logic/reducer.ts

import { GameState, Entity } from "./types";
import { Action } from "./actions";
import { resources } from "./allData";
import { removeMonsterFromCell } from "./utils"; // Импортируем вспомогательную функцию

export function gameReducer(state: GameState, action: Action): GameState {
  switch (action.type) {
    case "INITIALIZE_GAME":
      return {
        ...state,
        mode: action.payload.mode,
        instanceId: action.payload.instanceId,
        players: action.payload.players,
        inBattle: false,
        battleParticipants: null,
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
            const newHealth = Math.max(cell.monster.health - damage, 0);
            return newHealth > 0
              ? { ...cell, monster: { ...cell.monster, health: newHealth } }
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

    case "START_BATTLE":
      return {
        ...state,
        inBattle: true,
        battleParticipants: {
          attacker: action.payload.attacker,
          defender: action.payload.defender,
        },
      };

      case "END_BATTLE": {
        const { result, updatedAttacker, cellId } = action.payload;
        console.log(`END_BATTLE: result=${result}, updatedAttacker=`, updatedAttacker, `cellId=${cellId}`);
        
        const { attacker, defender } = state.battleParticipants!;
        let updatedState: GameState = { ...state, inBattle: false, battleParticipants: null };
  
        if (!updatedAttacker) {
          console.error("updatedAttacker is undefined in END_BATTLE action.");
          return state;
        }
  
        if (result === "attacker-win") {
          // Attacker победил, defender погиб
          if ("level" in updatedAttacker) { // Если атакующий - игрок
            console.log(`${updatedAttacker.name} победил в бою.`);
            updatedState = {
              ...updatedState,
              players: state.players.map(player =>
                player.id === updatedAttacker.id ? updatedAttacker : player
              ),
            };
            // Удаляем монстра из клетки
            if (cellId !== -1) {
              updatedState = removeMonsterFromCell(updatedState, cellId);
              console.log(`Монстр удалён из клетки с id=${cellId}.`);
            } else {
              console.warn("cellId для удаления монстра недействителен.");
            }
          } else { // Если атакующий - монстр
            console.log(`Монстр ${updatedAttacker.name} победил в бою.`);
            // Устанавливаем здоровье игрока (defender) на 0
            updatedState = {
              ...updatedState,
              players: state.players.map(player =>
                player.id === defender.id ? { ...player, health: 0 } : player
              ),
            };
          }
        } else if (result === "defender-win") {
          // Defender победил, attacker погиб
          if ("level" in defender) { // Если defender - игрок
            console.log(`${defender.name} победил в бою.`);
            // Удаляем монстра из клетки
            if (cellId !== -1) {
              updatedState = removeMonsterFromCell(updatedState, cellId);
              console.log(`Монстр удалён из клетки с id=${cellId}.`);
            } else {
              console.warn("cellId для удаления монстра недействителен.");
            }
          } else { // Если defender - монстр
            console.log(`Монстр ${defender.name} победил в бою.`);
            // Устанавливаем здоровье игрока (attacker) на 0
            updatedState = {
              ...updatedState,
              players: state.players.map(player =>
                player.id === attacker.id ? { ...player, health: 0 } : player
              ),
            };
          }
        }
  
        console.log("Updated State after END_BATTLE:", updatedState);
        return updatedState;
      }

    case "SET_MONSTERS_HAVE_ATTACKED":
      return { ...state, monstersHaveAttacked: action.payload.monstersHaveAttacked };

    case "TOGGLE_INVENTORY":
      return { ...state, inventoryOpen: !state.inventoryOpen };

    default:
      return state;
  }
}
