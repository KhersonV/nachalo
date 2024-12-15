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
  /**
 * Выполняет атаку монстров на игроков после завершения хода всех игроков.
 * 
 * 
 */

  const handleMonsterAttack = (
    player: PlayerState,
    monster: MonsterState
  ): PlayerState => {
    const damage = calculateDamage(monster.attack, player.defense);
    const newHealth = Math.max(0, player.health - damage);
  
    console.log(`до атаки hp = ${player.health}`);
    console.log(
      `Монстр ${monster.name} (ID=${monster.id}) атакует игрока ${player.name}: урон=${damage}, здоровье=${newHealth}`
    );
  
    if (newHealth === 0) {
      console.log(`Игрок ${player.name} погиб от атаки монстра ${monster.name}`);
    }
  
    // Возвращаем обновлённое состояние игрока
    return { ...player, health: newHealth };
  };



  /**
   * Выполняет атаку монстров на игроков после завершения хода всех игроков.
   */
 

const monstersAttackPlayers = () => {
  setState((prev) => {
    if (!prev.grid || !prev.players) return prev;

    // Копируем массив игроков
    const updatedPlayers = prev.players.map((player) => ({ ...player }));

    // Проходим по всем клеткам с монстрами
    prev.grid.forEach((cell) => {
      const { monster, x, y } = cell;
      if (!monster) return;

      updatedPlayers.forEach((player, index) => {
        if (
          player.health > 0 && // Игрок должен быть жив
          isPlayerInMonsterVision(player, monster, { x, y })
        ) {
          console.log(`до атаки hp = ${updatedPlayers[index].health}`);
          updatedPlayers[index] = handleMonsterAttack(updatedPlayers[index], monster); // Обновляем игрока
          console.log(`после атаки hp = ${updatedPlayers[index].health}`);
        }
      });
    });

    console.log("Обновленные игроки:", updatedPlayers);

    return {
      ...prev,
      players: updatedPlayers, // Возвращаем обновлённый массив игроков
    };
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
