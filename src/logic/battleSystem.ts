// battleSystem.ts

import { useGameContext } from "../components/GameContext";
import { GameState, PlayerState, Cell } from "../logic/types"; // Импортируем нужные типы

export function useBattleSystem() {
  const { state, setState } = useGameContext();

  function attackPlayerOrMonster(playerId: number, direction: { dx: number; dy: number }) {
    // Логика атаки
  }

  function openBarrel(playerId: number, direction: { dx: number; dy: number }) {
    // Логика открытия бочки
  }

  function tryExitThroughPortal(playerId: number) {
    setState((prev) => {
      if (!prev.grid) return prev;
      const player = prev.players.find((p) => p.id === playerId);
      if (!player) return prev;
      const cell = prev.grid.find((c) => c.x === player.position.x && c.y === player.position.y);
      if (!cell || !cell.isPortal) return prev;

      if (prev.artifactOwner === playerId) {
        console.log("Игрок вышел через портал с артефактом! Финализируем инстанс.");
      }

      return prev;
    });
  }

  function collectResourceIfOnTile(playerId: number) {
    setState((prev) => {
      if (!prev.grid) return prev;

      // Найти индекс игрока и его данные
      const playerIndex = prev.players.findIndex((p) => p.id === playerId);
      if (playerIndex === -1) return prev;

      const player = prev.players[playerIndex];

      // Найти клетку, на которой находится игрок
      const cell = prev.grid.find((c) => c.x === player.position.x && c.y === player.position.y);
      if (!cell) return prev;

      // Копируем инвентарь игрока
      const inventory = { ...player.inventory };

      if (cell.resource) {
        const resource = cell.resource;

        // Если это бочка
        if (resource.type === "barrbel") {
          console.log("Открываем бочку");

          // Пример добавления содержимого бочки
          addItemToInventory(inventory, "food", "/food-ground.webp", "Еда для выживания");

          // Убираем бочку с карты
          const newGrid = removeResourceFromGrid(prev.grid, cell.id);

          // Обновляем игрока и состояние
          const newPlayers = updatePlayer(prev.players, playerIndex, inventory, player);
          return { ...prev, players: newPlayers, grid: newGrid };
        }

        // Если это обычный ресурс
        addItemToInventory(inventory, resource.type, resource.image["ground"], resource.description);

        // Убираем ресурс с карты
        const newGrid = removeResourceFromGrid(prev.grid, cell.id);

        // Обновляем игрока и состояние
        const newPlayers = updatePlayer(prev.players, playerIndex, inventory, player);
        return { ...prev, players: newPlayers, grid: newGrid };
      }

      return prev;
    });
  }

  // Вспомогательные функции

  function addItemToInventory(
    inventory: Record<string, { count: number; image: string; description: string }>,
    type: string,
    image: string,
    description: string
  ): void {
    if (inventory[type]) {
      inventory[type] = {
        ...inventory[type], // Копируем существующий ресурс
        count: inventory[type].count + 1, // Увеличиваем только `count`
      };
    } else {
      inventory[type] = { count: 1, image: image || "/default-resource.webp", description };
    }
  }

  function removeResourceFromGrid(grid: Cell[], cellId: number): Cell[] {
    return grid.map((c) => (c.id === cellId ? { ...c, resource: null } : c));
  }

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

  return { attackPlayerOrMonster, openBarrel, tryExitThroughPortal, collectResourceIfOnTile };
}
