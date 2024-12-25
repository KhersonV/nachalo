// src/logic/reducer.ts

import { GameState, InventoryItem, Entity, PlayerState, Artifact } from "./types";
import { Action } from "./actions";
import { resources } from "./allData";
import { removeMonsterFromCell } from "./utils"; // какая-то ваша вспомогательная функция

function isPlayer(entity: Entity): entity is PlayerState {
  return "level" in entity;
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

    // ----------------------------------------
    // КЛЮЧЕВОЙ МОМЕНТ: логика конца боя
    // ----------------------------------------
    case "END_BATTLE": {
      const { result, updatedAttacker, cellId } = action.payload;
      const { attacker, defender } = state.battleParticipants || {};
      if (!attacker || !defender || !updatedAttacker) return state;

      let updatedState: GameState = {
        ...state,
        inBattle: false,
        battleParticipants: null,
      };

      // Проверим, кто победил
      if (result === "attacker-win" && isPlayer(updatedAttacker)) {
        // Если защитник тоже был игроком, то это PvP
        if (isPlayer(defender)) {
          console.log(
            `Игрок ${defender.name} погиб! У него есть артефакты?`,
            defender.inventory.artifacts
          );
          const loserArtifacts = defender.inventory.artifacts;
          
          // Если есть хотя бы один артефакт — предлагаем окно выбора
          if (Object.keys(loserArtifacts).length > 0) {
            // НЕ УДАЛЯЕМ проигравшего из массива, 
            // чтобы его инвентарь оставался доступным.
            updatedState = {
              ...updatedState,
              artifactSelection: {
                loserId: defender.id,    // ID проигравшего
                winnerId: updatedAttacker.id,
                // Внимание: складываем **InventoryItem** целиком
                artifacts: { ...loserArtifacts },
              },
            };
          } else {
            // Если артефактов нет, просто удаляем проигравшего
            updatedState = {
              ...updatedState,
              players: updatedState.players.filter((p) => p.id !== defender.id),
            };
          }
        }
        // Если защитник был монстром
        else {
          updatedState = removeMonsterFromCell(updatedState, cellId);
        }

        // Обновляем состояние победителя (у него уже может быть снижено здоровье и т.п.)
        updatedState.players = updatedState.players.map((p) =>
          p.id === updatedAttacker.id ? updatedAttacker : p
        );
      }
      // ---------------------------
      else if (result === "defender-win" && isPlayer(updatedAttacker)) {
        // Значит, проиграл attacker. Проверим, был ли он игроком
        if (isPlayer(attacker)) {
          console.log(
            `Игрок ${attacker.name} погиб! У него есть артефакты?`,
            attacker.inventory.artifacts
          );
          const loserArtifacts = attacker.inventory.artifacts;

          if (Object.keys(loserArtifacts).length > 0) {
            updatedState = {
              ...updatedState,
              artifactSelection: {
                loserId: attacker.id,
                winnerId: updatedAttacker.id,
                artifacts: { ...loserArtifacts },
              },
            };
          } else {
            updatedState = {
              ...updatedState,
              players: updatedState.players.filter((p) => p.id !== attacker.id),
            };
          }
        } else {
          updatedState = removeMonsterFromCell(updatedState, cellId);
        }

        // Обновляем победителя
        updatedState.players = updatedState.players.map((p) =>
          p.id === updatedAttacker.id ? updatedAttacker : p
        );
      }

      return updatedState;
    }
    // ----------------------------------------
    // ----------------------------------------

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
      const { winnerId, loserId, artifactKey } = action.payload;
      const { artifactSelection } = state;

      if (!artifactSelection) return state;

      // Находим проигравшего и победителя в массиве.
      const loserIndex = state.players.findIndex((p) => p.id === loserId);
      const winnerIndex = state.players.findIndex((p) => p.id === winnerId);
      if (loserIndex === -1 || winnerIndex === -1) {
        // На всякий случай
        return {
          ...state,
          artifactSelection: null,
        };
      }

      const loser = state.players[loserIndex];
      const winner = state.players[winnerIndex];

      // Ищем артефакт в инвентаре проигравшего
      const loserArtifacts = { ...loser.inventory.artifacts };
      const artifactItem = loserArtifacts[artifactKey];
      if (!artifactItem) {
        // Артефакт не найден
        return {
          ...state,
          artifactSelection: null,
        };
      }

      console.log(
        `Передаём артефакт "${artifactKey}" от ${loser.name} → ${winner.name}`
      );

      // Удаляем артефакт у проигравшего
      delete loserArtifacts[artifactKey];

      // Добавляем (или суммируем) у победителя
      const winnerArtifacts = { ...winner.inventory.artifacts };
      const existingWinnerItem = winnerArtifacts[artifactKey];
      if (existingWinnerItem) {
        // Если у победителя уже был этот ключ, суммируем
        existingWinnerItem.count += artifactItem.count;
      } else {
        winnerArtifacts[artifactKey] = { ...artifactItem }; // копируем весь объект
      }

      // Собираем обновлённые объекты
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

      // Теперь перезаписываем игроков в state.players
      let updatedPlayers = [...state.players];
      updatedPlayers[loserIndex] = updatedLoser;
      updatedPlayers[winnerIndex] = updatedWinner;

      // Допустим, мы хотим *после* передачи удалить проигравшего из игры.
      // Но только если он точно мёртв (health <= 0).
      // Если хотите убрать его независимо от здоровья — делайте без условия.
      if (updatedLoser.health <= 0) {
        updatedPlayers = updatedPlayers.filter((p) => p.id !== loserId);
      }

      return {
        ...state,
        players: updatedPlayers,
        artifactSelection: null, // закрываем диалог
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
