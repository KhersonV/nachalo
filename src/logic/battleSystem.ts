//battleSystem.ts

import { useGameContext } from "../components/GameContext";
import { MonsterState, PlayerState } from "../logic/types"; // Импортируем нужные типы

export function useBattleSystem() {
  const { state, setState } = useGameContext();

  /**
   * Рассчитывает урон с учётом защиты.
   */
  const calculateDamage = (attack: number, defense: number): number =>
    Math.max(0, attack - defense);

  /**
   * Проверяет, находится ли игрок в зоне видимости монстра.
   */
  const isPlayerInMonsterVision = (
    player: PlayerState,
    monster: MonsterState,
    monsterPosition: { x: number; y: number }
  ): boolean => {
    const distance =
      Math.abs(player.position.x - monsterPosition.x) +
      Math.abs(player.position.y - monsterPosition.y);
    return distance <= monster.vision;
  };

  /**
   * Обрабатывает атаку монстра на игрока.
   */
  const handleMonsterAttack = (
    player: PlayerState,
    monster: MonsterState
  ): PlayerState => {
    const damage = calculateDamage(monster.attack, player.defense);
    const newHealth = Math.max(0, player.health - damage);

    console.log(
      `Монстр ${monster.name} атакует игрока ${player.name}: урон=${damage}, здоровье=${newHealth}`
    );

    if (newHealth === 0) {
      console.log(`Игрок ${player.name} погиб от атаки монстра ${monster.name}`);
      // Здесь можно добавить дополнительные события, например, удаление игрока
    }

    return { ...player, health: newHealth };
  };

  /**
   * Выполняет атаку монстров на игроков после завершения хода всех игроков.
   */
  const monstersAttackPlayers = () => {
    setState((prev) => {
      if (!prev.grid || !prev.players) {
        return prev;
      }
  
      const updatedPlayers = [...prev.players];
      const attackedPlayers = new Set<number>();
  
      prev.grid.forEach((cell) => {
        const { monster, x, y } = cell;
        if (!monster) return;
  
        updatedPlayers.forEach((player, index) => {
          if (
            player.health > 0 &&
            !attackedPlayers.has(player.id) &&
            isPlayerInMonsterVision(player, monster, { x, y })
          ) {
            updatedPlayers[index] = handleMonsterAttack(player, monster);
            attackedPlayers.add(player.id);
          }
        });
      });
  
      return { ...prev, players: updatedPlayers };
    });
  };
  

  /**
   * Обрабатывает атаку игрока на монстра или другого игрока.
   */
  const attackPlayerOrMonster = (playerId: number, direction: { dx: number; dy: number }) => {
    console.log(`Игрок ${playerId} атакует в направлении (${direction.dx}, ${direction.dy})`);
    // Здесь будет ваша логика атаки
  };

  return {
    monstersAttackPlayers,
    attackPlayerOrMonster,
  };
}
