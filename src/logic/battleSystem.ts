// src/logic/battleSystem.ts

import { useGameContext } from "../components/GameContext";
import { MonsterState, PlayerState } from "./types";
import { Action } from "./actions";

export function useBattleSystem() {
  const { state, dispatch } = useGameContext();

  const calculateDamage = (attack: number, defense: number): number =>
    Math.max(0, attack - defense);

  const handleMonsterAttack = (player: PlayerState, monster: MonsterState): Action[] => {
    const damage = calculateDamage(monster.attack, player.defense);
    const newHealth = Math.max(0, player.health - damage);

    const actions: Action[] = [
      {
        type: 'ATTACK',
        payload: { attackerId: monster.id, targetId: player.id, damage },
      },
    ];

    if (newHealth === 0) {
      // Добавьте действие для обработки смерти игрока
      actions.push({ type: 'PLAYER_DIED', payload: { playerId: player.id } });
    }

    return actions;
  };

  const monstersAttackPlayers = () => {
    if (!state.grid) return;

    const actions: Action[] = [];

    state.grid.forEach((cell) => {
      if (cell.monster && cell.monster.type === 'aggressive') {
        state.players.forEach((player) => {
          const distance = Math.abs(player.position.x - cell.x) + Math.abs(player.position.y - cell.y);
          if (cell.monster && distance <= cell.monster.vision && player.health > 0) {
            actions.push(...handleMonsterAttack(player, cell.monster));
          }
        });
      }
    });

    actions.forEach(action => dispatch(action));
  };

  const attackPlayerOrMonster = (playerId: number, direction: { dx: number; dy: number }) => {
    const attacker = state.players.find(p => p.id === playerId);
    if (!attacker) return;

    const targetX = attacker.position.x + direction.dx;
    const targetY = attacker.position.y + direction.dy;

    const targetPlayer = state.players.find(p => p.position.x === targetX && p.position.y === targetY);
    const targetCell = state.grid?.find(c => c.x === targetX && c.y === targetY);

    if (targetPlayer) {
      // Атака игрока
      const damage = calculateDamage(attacker.attack, targetPlayer.defense);
      dispatch({ type: 'ATTACK', payload: { attackerId: playerId, targetId: targetPlayer.id, damage } });
    } else if (targetCell?.monster) {
      // Атака монстра
      const damage = calculateDamage(attacker.attack, targetCell.monster.defense);
      dispatch({ type: 'ATTACK', payload: { attackerId: playerId, targetId: targetCell.monster.id, damage } });
      // Дополнительно: обработка состояния монстра, если требуется
    }
  };

  return {
    monstersAttackPlayers,
    attackPlayerOrMonster,
  };
}
