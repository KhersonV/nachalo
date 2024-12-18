// src/logic/battleSystem.ts

import { useGameContext } from "../components/GameContext";
import { Entity } from "./types";

export function useBattleSystem() {
  const { state, dispatch } = useGameContext();

  const monstersAttackPlayers = () => {
    if (!state.grid) return;

    state.grid.forEach((cell) => {
      if (cell.monster && cell.monster.type === 'aggressive') {
        state.players.forEach((player) => {
          const distance = Math.abs(player.position.x - cell.x) + Math.abs(player.position.y - cell.y);
          if (cell.monster && distance <= cell.monster.vision && player.health > 0) {
            // Инициируем бой
            const attacker: Entity = cell.monster!;
            const defender: Entity = player;

            if (attacker.id === defender.id) {
              console.warn(`Игрок ${attacker.name} атакует сам себя!`);
              return;
            }
            console.log(`Бой инициирован: ${attacker.name} атакует ${defender.name}`);
            dispatch({ type: 'START_BATTLE', payload: { attacker, defender } });
          }
        });
      }
    });
  };

  const attackPlayerOrMonster = (playerId: number, direction: { dx: number; dy: number }) => {
    const attacker = state.players.find(p => p.id === playerId);
    if (!attacker) return;

    const targetX = attacker.position.x + direction.dx;
    const targetY = attacker.position.y + direction.dy;

    const targetPlayer = state.players.find(p => p.position.x === targetX && p.position.y === targetY);
    const targetCell = state.grid?.find(c => c.x === targetX && c.y === targetY);

    if (targetPlayer && targetPlayer.id !== attacker.id) { // Убедимся, что атакуем другого игрока
      // Инициируем бой с другим игроком
      const defender: Entity = targetPlayer;
      dispatch({ type: 'START_BATTLE', payload: { attacker, defender } });
    } else if (targetCell?.monster) {
      // Инициируем бой с монстром (агрессивным или нейтральным)
      const defender: Entity = targetCell.monster!;
      dispatch({ type: 'START_BATTLE', payload: { attacker, defender } });
    }
  };

  return {
    monstersAttackPlayers,
    attackPlayerOrMonster,
  };
}
