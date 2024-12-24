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
        let updatedState: GameState = { 
          ...state, 
          inBattle: false, 
          battleParticipants: null 
        };
      
        if (!updatedAttacker) {
          console.error("updatedAttacker is undefined in END_BATTLE action.");
          return state;
        }
      
        if (result === "attacker-win") {
          // Победил АТАКУЮЩИЙ (и это сущность, которую мы передали как updatedAttacker)
      
          if ("level" in updatedAttacker) {
            // updatedAttacker – Игрок
            console.log(`Победил игрок ${updatedAttacker.name}. Ему уже начислили XP в BattleScene.`);
            
            // Обновим игрока (победителя) в списке игроков
            updatedState = {
              ...updatedState,
              players: state.players.map(player =>
                player.id === updatedAttacker.id ? updatedAttacker : player
              ),
            };
      
            // Проигравший – defender
            if (!("level" in defender)) {
              // Защитник – монстр ⇒ убираем монстра с карты (cellId)
              if (cellId !== -1) {
                updatedState = removeMonsterFromCell(updatedState, cellId);
                console.log(`Монстр ${defender.name} удалён из клетки id=${cellId}.`);
              }
            } else {
              // Защитник – игрок ⇒ ставим ему HP=0 или убираем из players (ваша логика)
              updatedState = {
                ...updatedState,
                players: updatedState.players.map(p =>
                  p.id === defender.id ? { ...p, health: 0 } : p
                ),
              };
            }
      
          } else {
            // updatedAttacker – монстр
            console.log(`Победил монстр ${updatedAttacker.name}. Проигравший - игрок attacker.`);
      
            // Значит, надо обнулить здоровье игрока, который проиграл
            updatedState = {
              ...updatedState,
              players: updatedState.players.map(player =>
                player.id === defender.id ? { ...player, health: 0 } : player
              ),
            };
          }
        }
      
        else if (result === "defender-win") {
          // Победил ЗАЩИТНИК (и он передан в updatedAttacker)
          if ("level" in updatedAttacker) {
            // updatedAttacker – Игрок, значит он выиграл
            console.log(`Победил игрок ${updatedAttacker.name}.`);
      
            // Обновим игрока (победителя)
            updatedState = {
              ...updatedState,
              players: state.players.map(player =>
                player.id === updatedAttacker.id ? updatedAttacker : player
              ),
            };
      
            // Проигравший – attacker
            if (!("level" in attacker)) {
              // attacker – монстр ⇒ убираем с карты
              if (cellId !== -1) {
                updatedState = removeMonsterFromCell(updatedState, cellId);
                console.log(`Монстр ${attacker.name} удалён из клетки id=${cellId}.`);
              }
            } else {
              // attacker – игрок ⇒ ставим ему HP=0 или убираем из массива
              updatedState = {
                ...updatedState,
                players: updatedState.players.map(p =>
                  p.id === attacker.id ? { ...p, health: 0 } : p
                ),
              };
            }
          } else {
            // updatedAttacker – монстр, значит он выиграл
            console.log(`Победил монстр ${updatedAttacker.name}. Проигравший - игрок attacker.`);
            updatedState = {
              ...updatedState,
              players: state.players.map(p =>
                p.id === attacker.id ? { ...p, health: 0 } : p
              ),
            };
          }
        }
      
        console.log("Updated State after END_BATTLE:", updatedState);
        return updatedState;
      }

      case "REMOVE_RESOURCE": {
        const { cellId } = action.payload;
        // Убираем resource с клетки (cell)
        const updatedGrid = state.grid.map((cell) =>
          cell.id === cellId ? { ...cell, resource: null } : cell
        );
        return { ...state, grid: updatedGrid };
      }
  
      // ----- Добавляем обычный ресурс (счётчик count) -----
      case "ADD_RESOURCE": {
        const { playerId, resourceType, description, image } = action.payload;
        const updatedPlayers = state.players.map((player) => {
          if (player.id !== playerId) return player;
  
          // Находим предыдущую запись в инвентаре (если есть).
          const existingItem = player.inventory[resourceType] || { count: 0 };
  
          return {
            ...player,
            inventory: {
              ...player.inventory,
              [resourceType]: {
                ...existingItem,
                count: (existingItem.count || 0) + 1,
                description,
                image,
              },
            },
          };
        });
  
        return { ...state, players: updatedPlayers };
      }
  
      // ----- Добавляем артефакт (уникальная вещь) -----
      // Можно тоже вести счётчик count=1, если хотите разрешить
      // несколько одинаковых артефактов. Или всегда 1.
      case "ADD_ARTIFACT": {
        const { playerId, artifactName, description, image, bonus } = action.payload;
        const updatedPlayers = state.players.map((player) => {
          if (player.id !== playerId) return player;
  
          // Пусть в inventory хранятся и артефакты, и ресурсы.
          // Но артефакты чаще всего имеют count=1 (или вообще не используют count).
          const existingItem = player.inventory[artifactName] || { count: 0 };
  
          return {
            ...player,
            inventory: {
              ...player.inventory,
              [artifactName]: {
                ...existingItem,
                count: (existingItem.count || 0) + 1, // при необходимости
                description,
                image,
                bonus,
              },
            },
          };
        });
  
        return { ...state, players: updatedPlayers };
      }
  

    case "SET_MONSTERS_HAVE_ATTACKED":
      return { ...state, monstersHaveAttacked: action.payload.monstersHaveAttacked };

    case "TOGGLE_INVENTORY":
      return { ...state, inventoryOpen: !state.inventoryOpen };

    default:
      return state;
  }
}
