// src/logic/battleSystem.ts

import { useGameContext } from "../components/GameContext";
import { Entity, GameState} from "./types";
import { Action } from "./actions";
import { useCallback } from "react";

export function useBattleSystem() {
  const { state, dispatch } = useGameContext();

  const monstersAttackPlayers = useCallback(() => {
    if (!state.grid || state.inBattle) return; // Проверка на уже идущий бой

    for (const cell of state.grid) {
      if (cell.monster && cell.monster.type === 'aggressive') {
        for (const player of state.players) {
          const distance = Math.abs(player.position.x - cell.x) + Math.abs(player.position.y - cell.y);
          if (distance <= cell.monster.vision && player.health > 0) {
            // Инициируем бой только если монстр всё ещё жив
            if (cell.monster.health > 0) {
              const attacker: Entity = cell.monster;
              const defender: Entity = player;

              // Проверка, что атакующий не атакует себя
              if (attacker.id === defender.id) {
                console.warn(`Игрок ${attacker.name} атакует сам себя!`);
                continue;
              }

              // Логирование для отладки
              console.log(`Инициирован бой: ${attacker.name} атакует ${defender.name}`);

              const cellId: number = cell.id; // Получаем cellId

              dispatch({ type: 'START_BATTLE', payload: { attacker, defender, cellId } });

              // Прерываем цикл, чтобы не инициировать несколько боев одновременно
              return;
            }
          }
        }
      }
    }
  }, [state.grid, state.inBattle, state.players, dispatch]);

  const attackPlayerOrMonster = useCallback((playerId: number, direction: { dx: number; dy: number }) => {
    if (state.inBattle) return; // Запрет на атаку, если уже идет бой

    const attacker = state.players.find(p => p.id === playerId);
    if (!attacker) return;

    const targetX = attacker.position.x + direction.dx;
    const targetY = attacker.position.y + direction.dy;

    const targetPlayer = state.players.find(p => p.position.x === targetX && p.position.y === targetY);
    const targetCell = state.grid?.find(c => c.x === targetX && c.y === targetY);

    if (targetPlayer && targetPlayer.id !== attacker.id) { // Убедимся, что атакуем другого игрока
      // Инициируем бой с другим игроком
      const defender: Entity = targetPlayer;
      const cellId: number = findCellId(state, defender); // Функция для поиска cellId

      console.log(`Игрок ${attacker.name} атакует игрока ${defender.name}`);

      dispatch({ type: 'START_BATTLE', payload: { attacker, defender, cellId } });
    } else if (targetCell?.monster) {
      // Инициируем бой с монстром (агрессивным или нейтральным)
      const defender: Entity = targetCell.monster;
      const cellId: number = targetCell.id;

      console.log(`Игрок ${attacker.name} атакует монстра ${defender.name}`);

      dispatch({ type: 'START_BATTLE', payload: { attacker, defender, cellId } });
    }
  }, [state.inBattle, state.players, state.grid, dispatch]);

  return {
    monstersAttackPlayers,
    attackPlayerOrMonster,
  };
}

// Вспомогательная функция для поиска cellId
function findCellId(state: GameState, entity: Entity): number {
  const cell = state.grid.find(cell => cell.monster && cell.monster.id === entity.id);
  return cell ? cell.id : -1;
}
