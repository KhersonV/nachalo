// src/logic/battleSystem.ts

import { useGameContext } from "../components/GameContext";
import { Entity } from "./types";

export function useBattleSystem() {
  const { state, dispatch } = useGameContext();

  const monstersAttackPlayers = () => {
    if (!state.grid || state.inBattle) return; // Проверка на уже идущий бой

    for (const cell of state.grid) {
      if (cell.monster && cell.monster.type === 'aggressive') {
        for (const player of state.players) {
          const distance = Math.abs(player.position.x - cell.x) + Math.abs(player.position.y - cell.y);
          if (distance <= cell.monster.vision && player.health > 0) {
            // Инициируем бой
            const attacker: Entity = cell.monster!;
            const defender: Entity = player;

            // Проверка, что атакующий не атакует себя
            if (attacker.id === defender.id) {
              console.warn(`Игрок ${attacker.name} атакует сам себя!`);
              continue;
            }

            // Логирование для отладки
            console.log(`Инициирован бой: ${attacker.name} атакует ${defender.name}`);

            dispatch({ type: 'START_BATTLE', payload: { attacker, defender } });

            // Прерываем цикл, чтобы не инициировать несколько боев одновременно
            return;
          }
        }
      }
    }
  };

  const attackPlayerOrMonster = (playerId: number, direction: { dx: number; dy: number }) => {
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
      console.log(`Игрок ${attacker.name} атакует игрока ${defender.name}`);
      dispatch({ type: 'START_BATTLE', payload: { attacker, defender } });
    } else if (targetCell?.monster) {
      // Инициируем бой с монстром (агрессивным или нейтральным)
      const defender: Entity = targetCell.monster!;
      console.log(`Игрок ${attacker.name} атакует монстра ${defender.name}`);
      dispatch({ type: 'START_BATTLE', payload: { attacker, defender } });
    }
  };

  return {
    monstersAttackPlayers,
    attackPlayerOrMonster,
  };
}
