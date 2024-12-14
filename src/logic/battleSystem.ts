//battleSystem.ts

import { useGameContext } from "../components/GameContext";
import { MonsterState, PlayerState, Cell } from "../logic/types"; // Импортируем нужные типы

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
      if (!prev.grid || !prev.players){
       return prev;
      }
      // Создаем копии игроков для изменений
      const updatedPlayers = [...prev.players];

      // Обрабатываем каждую клетку с монстром
      prev.grid.forEach((cell: Cell) => {
        const { monster, x, y } = cell;
        if (!monster) return;

        // Находим всех игроков в зоне видимости монстра
        updatedPlayers.forEach((player, index) => {
          if (
            player.health > 0 && // Игрок должен быть жив
            isPlayerInMonsterVision(player, monster, { x, y })
          ) {
            updatedPlayers[index] = handleMonsterAttack(player, monster);
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

  /**
   * Обрабатывает взаимодействие с бочкой.
   */
  const openBarrel = (playerId: number, direction: { dx: number; dy: number }) => {
    console.log(`Игрок ${playerId} пытается открыть бочку в направлении (${direction.dx}, ${direction.dy})`);
    // Здесь будет ваша логика взаимодействия с бочкой
  };

  /**
   * Обрабатывает попытку выхода через портал.
   */
  const tryExitThroughPortal = (playerId: number) => {
    setState((prev) => {
      if (!prev.grid) return prev;
      const player = prev.players.find((p) => p.id === playerId);
      if (!player) return prev;

      const cell = prev.grid.find(
        (c) => c.x === player.position.x && c.y === player.position.y
      );
      if (!cell || !cell.isPortal) return prev;

      if (prev.artifactOwner === playerId) {
        console.log(`Игрок ${player.name} вышел через портал с артефактом! Финализируем инстанс.`);
        // Здесь можно вызвать функцию завершения инстанса
      }

      return prev;
    });
  };

  /**
   * Обрабатывает сбор ресурса с клетки.
   */
  const collectResourceIfOnTile = (playerId: number) => {
    setState((prev) => {
      if (!prev.grid) return prev;

      const playerIndex = prev.players.findIndex((p) => p.id === playerId);
      if (playerIndex === -1) return prev;

      const player = prev.players[playerIndex];
      const cell = prev.grid.find(
        (c) => c.x === player.position.x && c.y === player.position.y
      );
      if (!cell) return prev;

      const inventory = { ...player.inventory };

      if (cell.resource) {
        const resource = cell.resource;

        // Если это бочка
        if (resource.type === "barrbel") {
          console.log("Открываем бочку");
          addItemToInventory(inventory, "food", "/food-ground.webp", "Еда для выживания");

          const newGrid = removeResourceFromGrid(prev.grid, cell.id);
          const newPlayers = updatePlayer(prev.players, playerIndex, inventory, player);
          return { ...prev, players: newPlayers, grid: newGrid };
        }

        // Если это обычный ресурс
        addItemToInventory(inventory, resource.type, resource.image["ground"], resource.description);
        const newGrid = removeResourceFromGrid(prev.grid, cell.id);
        const newPlayers = updatePlayer(prev.players, playerIndex, inventory, player);
        return { ...prev, players: newPlayers, grid: newGrid };
      }

      return prev;
    });
  };

  /**
   * Добавляет предмет в инвентарь.
   */
  function addItemToInventory(
    inventory: Record<string, { count: number; image: string; description: string }>,
    type: string,
    image: string,
    description: string
  ): void {
    if (inventory[type]) {
      inventory[type] = {
        ...inventory[type],
        count: inventory[type].count + 1,
      };
    } else {
      inventory[type] = { count: 1, image: image || "/default-resource.webp", description };
    }
  }

  /**
   * Убирает ресурс с карты.
   */
  function removeResourceFromGrid(grid: Cell[], cellId: number): Cell[] {
    return grid.map((c) => (c.id === cellId ? { ...c, resource: null } : c));
  }

  /**
   * Обновляет игрока в списке игроков.
   */
  function updatePlayer(
    players: PlayerState[],
    playerIndex: number,
    inventory: Record<string, { count: number; image: string; description: string }>,
    player: PlayerState
  ): PlayerState[] {
    const updatedPlayers = [...players];
    updatedPlayers[playerIndex] = {
      ...player,
      inventory,
      energy: Math.max(0, player.energy - 1),
    };
    return updatedPlayers;
  }

  return {
    monstersAttackPlayers,
    attackPlayerOrMonster,
    openBarrel,
    tryExitThroughPortal,
    collectResourceIfOnTile,
  };
}
