
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//************************************************* src/logic/reducer.ts ******************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************


import { GameState, InventoryItem, Entity, PlayerState, Artifact } from "./types";
import { Action } from "./actions";
import { resources } from "./allData";
import { removeMonsterFromCell } from "./utils"; // какая-то ваша вспомогательная функция

function isPlayer(entity: Entity): entity is PlayerState {
  return "inventory" in entity;
}

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
      const {  targetId, damage, targetType, cellId } = action.payload;

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
      if (!resources[resourceType]) return state; // Если ресурс неизвестен, выходим
    
      const updatedPlayers = state.players.map((player) => {
        if (player.id !== playerId) return player;
    
        // Допустим, мы решили, что все "обычные" ресурсы храним в player.inventory.resources
        return {
          ...player,
          inventory: {
            ...player.inventory,
            resources: {
              ...player.inventory.resources,
              [resourceType]: {
                // Берём старый count, если есть
                count: (player.inventory.resources[resourceType]?.count || 0) + 1,
                description: resources[resourceType].description,
                image: `/main_resources/${resourceType}.webp`,
              },
            },
          },
        };
      });
    
      // Удаляем ресурс с карты
      const updatedGrid = state.grid.map((cell) =>
        cell.id === cellId ? { ...cell, resource: null } : cell
      );
    
      return { ...state, players: updatedPlayers, grid: updatedGrid };
    }

    case "USE_ITEM": {
      const { playerId, itemType } = action.payload;
    
      const updatedPlayers = state.players.map((player) => {
        if (player.id !== playerId) return player;
    
        const item = player.inventory.resources[itemType];
        if (!item || item.count <= 0) {
          // Ресурса нет, ничего не делаем
          return player;
        }
    
        // Применяем эффект
        const resourceEffect = resources[itemType]?.effect || 0;
    
        return {
          ...player,
          inventory: {
            ...player.inventory,
            resources: {
              ...player.inventory.resources,
              [itemType]: {
                // Уменьшаем count
                ...item,
                count: item.count - 1,
              },
            },
          },
          // Если это "food" → восстанавливаем здоровье, 
          // если "water" → восстанавливаем энергию, и т. п.
          ...(itemType === "food"
            ? {
                health: Math.min(player.maxHealth, player.health + resourceEffect),
              }
            : {}),
          ...(itemType === "water"
            ? {
                energy: Math.min(player.maxEnergy, player.energy + resourceEffect),
              }
            : {}),
        };
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

    case "START_BATTLE":
      return {
        ...state,
        inBattle: true,
        battleParticipants: {
          attacker: action.payload.attacker,
          defender: action.payload.defender,
        },
      };


      ///***************************************************************
      ///***************************************************************
      ///***************************************************************
      ///***************************************************************
      ///***************************************************************
      ///***************************************************************

    // ----------------------------------------
    // КЛЮЧЕВОЙ МОМЕНТ: логика конца боя
    // ----------------------------------------
    case "END_BATTLE": {
      const { result, updatedAttacker, updatedDefender, cellId } = action.payload;
      const { attacker, defender } = state.battleParticipants || {};
      if (!attacker || !defender) return state;
    
      let updatedState: GameState = {
        ...state,
        inBattle: false,
        battleParticipants: null,
      };
    
      // Обновляем здоровье атакующего, если он игрок
      if (isPlayer(updatedAttacker)) {
        updatedState.players = updatedState.players.map((p) =>
          p.id === updatedAttacker.id ? { ...p, ...updatedAttacker } : p
        );
      } else {
        // Или если это монстр
        if (updatedAttacker.health <= 0) {
          updatedState = removeMonsterFromCell(updatedState, cellId);
        } else {
          updatedState.grid = updatedState.grid.map((c) => {
            if (c.id === cellId && c.monster && c.monster.id === updatedAttacker.id) {
              return {
                ...c,
                monster: { ...c.monster, health: updatedAttacker.health },
              };
            }
            return c;
          });
        }
      }
    
      // Обновляем здоровье защитника, если он игрок
      if (isPlayer(updatedDefender)) {
        updatedState.players = updatedState.players.map((p) =>
          p.id === updatedDefender.id ? { ...p, ...updatedDefender } : p
        );
      } else {
        if (updatedDefender.health <= 0) {
          updatedState = removeMonsterFromCell(updatedState, cellId);
        } else {
          updatedState.grid = updatedState.grid.map((c) => {
            if (c.id === cellId && c.monster && c.monster.id === updatedDefender.id) {
              return {
                ...c,
                monster: { ...c.monster, health: updatedDefender.health },
              };
            }
            return c;
          });
        }
      }
    
      // Удаляем проигравшего игрока с нулевым здоровьем
      if (result === "attacker-win" && isPlayer(updatedDefender)) {
        if (updatedDefender.health <= 0) {
          console.log(`Удаляем игрока ${updatedDefender.name} с нулевым здоровьем.`);
          updatedState.players = updatedState.players.filter((p) => p.id !== updatedDefender.id);
        }
      } else if (result === "defender-win" && isPlayer(updatedAttacker)) {
        if (updatedAttacker.health <= 0) {
          console.log(`Удаляем игрока ${updatedAttacker.name} с нулевым здоровьем.`);
          updatedState.players = updatedState.players.filter((p) => p.id !== updatedAttacker.id);
        }
      }
    
      return updatedState;
    }
    

     ///***************************************************************
     ///***************************************************************
     ///***************************************************************
     ///***************************************************************
     ///***************************************************************

    case "REMOVE_PLAYER": {
      const { playerId } = action.payload;
      return {
        ...state,
        players: state.players.filter((player) => player.id !== playerId),
      };
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
      return {
        ...state,
        players: state.players.map((player) => {
          if (player.id !== playerId) return player;

          // берем прежнюю запись, если есть
          const existingItem =
            player.inventory.resources[resourceType] || { count: 0, description, image };

          return {
            ...player,
            inventory: {
              ...player.inventory,
              resources: {
                ...player.inventory.resources,
                [resourceType]: {
                  ...existingItem,
                  count: (existingItem.count || 0) + 1,
                  description,
                  image,
                },
              },
            },
          };
        }),
      };
    }

    // ----- Добавляем артефакт (но в виде InventoryItem) -----
    case "ADD_ARTIFACT": {
      const { playerId, artifactName, description, image, bonus } = action.payload;
      return {
        ...state,
        players: state.players.map((player) => {
          if (player.id !== playerId) return player;

          const existingItem = player.inventory.artifacts[artifactName] || {
            count: 0,
            description,
            image,
            bonus,
          };

          return {
            ...player,
            inventory: {
              ...player.inventory,
              artifacts: {
                ...player.inventory.artifacts,
                [artifactName]: {
                  ...existingItem,
                  count: (existingItem.count || 0) + 1,
                  description,
                  image,
                  bonus,
                },
              },
            },
          };
        }),
      };
    }

    // Начинаем выбор артефактов (если нужно вызвать вручную):
    case "START_ARTIFACT_SELECTION": {
      return {
        ...state,
        artifactSelection: {
          loserId: action.payload.loserId,
          winnerId: action.payload.winnerId,
          // Обратите внимание: здесь у нас тип Record<string, Artifact>. 
          // Но мы можем хранить именно InventoryItem, если так удобнее.
          // Сейчас по коду - у нас ArtifactSelection ожидает InventoryItem,
          // поэтому для примера предположим, что вы где-то сконвертировали их.
          artifacts: {},
        },
      };
    }

    // Отмена выбора (не переносим ничего)
    case "CANCEL_ARTIFACT_SELECTION": {
      return {
        ...state,
        artifactSelection: null,
      };
    }

    // -------------------------------------------------------
    // КЛЮЧЕВОЙ экшен - перенос выбранного артефакта от проигравшего к победителю
    // -------------------------------------------------------
    
    case "COMPLETE_ARTIFACT_SELECTION": {
      let updatedPlayers = [...state.players];
    
      const { winnerId, loserId, artifactKey } = action.payload;
      const loserIndex = state.players.findIndex((p) => p.id === loserId);
      const winnerIndex = state.players.findIndex((p) => p.id === winnerId);
    
      if (loserIndex === -1 || winnerIndex === -1) {
        return {
          ...state,
          artifactSelection: null,
        };
      }
    
      const loser = updatedPlayers[loserIndex];
      const winner = updatedPlayers[winnerIndex];
    
      const loserArtifacts = { ...loser.inventory.artifacts };
      const artifactItem = loserArtifacts[artifactKey];
      if (!artifactItem) {
        return {
          ...state,
          artifactSelection: null,
        };
      }
    
      // Удаляем артефакт у проигравшего
      delete loserArtifacts[artifactKey];
    
      // Добавляем артефакт победителю
      const winnerArtifacts = { ...winner.inventory.artifacts };
      if (winnerArtifacts[artifactKey]) {
        winnerArtifacts[artifactKey].count += artifactItem.count;
      } else {
        winnerArtifacts[artifactKey] = { ...artifactItem };
      }
    
      const updatedLoser = {
        ...loser,
        inventory: {
          ...loser.inventory,
          artifacts: loserArtifacts,
        },
      };
    
      const updatedWinner = {
        ...winner,
        inventory: {
          ...winner.inventory,
          artifacts: winnerArtifacts,
        },
      };
    
      updatedPlayers[loserIndex] = updatedLoser;
      updatedPlayers[winnerIndex] = updatedWinner;
    
      // Удаляем проигравшего игрока, если его здоровье <= 0
      if (updatedLoser.health <= 0) {
        updatedPlayers = updatedPlayers.filter((p) => p.id !== loserId);
      }
    
      // Сбрасываем индекс текущего игрока, если он удалён
      let newCurrentPlayerIndex = state.currentPlayerIndex;
      if (newCurrentPlayerIndex >= updatedPlayers.length) {
        newCurrentPlayerIndex = 0;
      }
    
      return {
        ...state,
        players: updatedPlayers,
        currentPlayerIndex: newCurrentPlayerIndex,
        artifactSelection: null,
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
