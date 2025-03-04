
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

const usedTransactionIds = new Set<string>();


function isPlayer(entity: Entity): entity is PlayerState {
  return "inventory" in entity;
}

function endBattleSwitchCurrentPlayer(updatedState: GameState): GameState {
  const stillHasCurrent = updatedState.players.some(
    (p) => p.id === updatedState.currentPlayerId
  );
  if (!stillHasCurrent) {
    if (updatedState.players.length === 0) {
      updatedState.currentPlayerId = -1; // или null
    } else {
      updatedState.currentPlayerId = updatedState.players[0].id;
    }
  }
  return updatedState;
}

function generateTransactionId() {
  // Можно GUID, Date.now() + Math.random(), nanoid и т.п.
  return "tx_" + Math.floor(Math.random() * 1_000_000_000);
}

export function gameReducer(state: GameState, action: Action): GameState {
  switch (action.type) {
    case "INITIALIZE_GAME":
      const [firstPlayer] = action.payload.players;
      return {
        ...state,
        mode: action.payload.mode,
        instanceId: action.payload.instanceId,
        players: action.payload.players,
        inBattle: false,
        battleParticipants: null,
        currentPlayerId: firstPlayer?.id ?? 0,
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
      const { targetId, damage, targetType, cellId } = action.payload;

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
      if (state.players.length === 0) {
        return state; // Нет игроков
      }
      // 1) Ищем, где находится текущий игрок среди state.players
      const currentIndex = state.players.findIndex(p => p.id === state.currentPlayerId);
      if (currentIndex === -1) {
        // Текущий игрок не найден — берём 0
        const newId = state.players[0].id;
        return { ...state, currentPlayerId: newId };
      }

      // 2) Следующий индекс
      const nextIndex = (currentIndex + 1) % state.players.length;
      const nextPlayerId = state.players[nextIndex].id;

      let updatedState = {
        ...state,
        currentPlayerId: nextPlayerId,
      };
      // Если вернулись на 0-й индекс, значит начался новый цикл
      if (nextIndex === 0) {
        updatedState = {
          ...updatedState,
          turnCycle: state.turnCycle + 1,
          monstersHaveAttacked: false,
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
    
      // Выходим из боя
      let updatedState: GameState = {
        ...state,
        inBattle: false,
        battleParticipants: null,
      };
    
      // 1) Обновляем здоровье/статус атакующего (если это игрок)
      if (isPlayer(updatedAttacker)) {
        // Если это игрок — применяем обновлённые HP
        updatedState.players = updatedState.players.map((p) =>
          p.id === updatedAttacker.id ? { ...p, ...updatedAttacker } : p
        );
      } else {
        // Если это монстр
        if (updatedAttacker.health <= 0) {
          updatedState = removeMonsterFromCell(updatedState, cellId);
        } else {
          // Обновить здоровье монстра
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
    
      // 2) Обновляем защитника (если это игрок)
      if (isPlayer(updatedDefender)) {
        updatedState.players = updatedState.players.map((p) =>
          p.id === updatedDefender.id ? { ...p, ...updatedDefender } : p
        );
      } else {
        // Монстр
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
    
      // ------------------------------
      // 3) Определяем loser / winner
      // ------------------------------
      let loser: Entity | null = null;
      let winner: Entity | null = null;
    
      if (result === "attacker-win") {
        loser = updatedDefender;
        winner = updatedAttacker;
      } else if (result === "defender-win") {
        loser = updatedAttacker;
        winner = updatedDefender;
      }
    
      // Если нет чёткого результата (или кто-то умер одновременно?),
      // но для упрощения допустим, что всегда есть loser/winner
      if (!loser || !winner) {
        // Переключаем ход и выходим
        return endBattleSwitchCurrentPlayer(updatedState);
      }
    
      // 4) Если проигравший и победитель — оба игроки:
      if (isPlayer(loser) && isPlayer(winner)) {
        // Копия артефактов проигравшего
        const copyArtifacts = { ...loser.inventory.artifacts };
        const hasArtifacts = Object.keys(copyArtifacts).length > 0;
    
        // Удаляем проигравшего игрока сразу
        updatedState.players = updatedState.players.filter((p) => p.id !== loser.id);
    
        // Если у проигравшего *есть* артефакты — записываем их в artifactSelection
        if (hasArtifacts) {
          updatedState = {
            ...updatedState,
            artifactSelection: {
              loserId: loser.id,
              winnerId: winner.id,
              artifacts: copyArtifacts,
            },
          };
        }
      } else {
        // Иначе (если монстр против игрока) — ничего не передаём.
        // Если проигравший — игрок, можем удалить его, если health <= 0,
        // НО вы, возможно, уже обновили его HP выше
        if (isPlayer(loser) && loser.health <= 0) {
          updatedState.players = updatedState.players.filter((p) => p.id !== loser.id);
        }
      }
    
      // 5) Переключаем ход, если текущий игрок удалён
      return endBattleSwitchCurrentPlayer(updatedState);
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
      // 1) Доп. защита: если artifactSelection уже сброшен => ничего не делаем
      if (!state.artifactSelection) {
        return state; 
      }

      const { winnerId, loserId, artifactKey } = action.payload;
      console.log("REDUCER => COMPLETE_ARTIFACT_SELECTION");
      console.log("   winnerId:", winnerId);
      console.log("   loserId:", loserId);
      console.log("   artifactKey:", artifactKey);

      // 2) Достаём копию артефактов проигравшего (из state.artifactSelection)
      const loserArtifacts = { ...state.artifactSelection.artifacts };
      const artifactItem = loserArtifacts[artifactKey];
      if (!artifactItem) {
        console.log("   ERROR: no artifactItem found => key:", artifactKey);
        return {
          ...state,
          artifactSelection: null,
        };
      }
      console.log("   artifactItem to transfer:", artifactItem);

      // 3) Ищем победителя
      const updatedPlayers = [...state.players];
      const winnerIndex = updatedPlayers.findIndex((p) => p.id === winnerId);
      if (winnerIndex === -1) {
        console.log("   ERROR: winner not found, id=", winnerId);
        return {
          ...state,
          artifactSelection: null,
        };
      }

      // 4) Добавляем к победителю
      const winner = updatedPlayers[winnerIndex];
      const winnerArtifacts = { ...winner.inventory.artifacts };

      // Суммируем count
      if (winnerArtifacts[artifactKey]) {
        winnerArtifacts[artifactKey].count += artifactItem.count;
      } else {
        winnerArtifacts[artifactKey] = { ...artifactItem };
      }

      updatedPlayers[winnerIndex] = {
        ...winner,
        inventory: {
          ...winner.inventory,
          artifacts: winnerArtifacts,
        },
      };

      // 5) Сбрасываем artifactSelection
      return {
        ...state,
        players: updatedPlayers,
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
