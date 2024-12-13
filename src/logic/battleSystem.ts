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

      const playerIndex = prev.players.findIndex((p) => p.id === playerId);
      if (playerIndex === -1) return prev;

      const player = prev.players[playerIndex];
      const cell = prev.grid.find((c) => c.x === player.position.x && c.y === player.position.y);
      if (!cell) return prev;

      const newPlayers = [...prev.players];
      const inventory = { ...player.inventory };

      // Проверка, является ли ресурс бочкой
      if (cell.resource?.type === "barrbel") {
        console.log("Открываем бочку");

        // Пример добавления содержимого бочки в инвентарь
        addItemToInventory(inventory, "food", "/food-ground.webp", "Еда для выживания");

        // Убираем бочку с карты
        const newGrid = removeResourceFromGrid(prev.grid, cell.id);
        return { ...prev, players: updatePlayer(newPlayers, playerIndex, inventory, player), grid: newGrid };
      }

      // Если ресурс обычный
      if (cell.resource) {
        const resource = cell.resource;
        addItemToInventory(inventory, resource.type, resource.image["ground"], resource.description);

        const newGrid = removeResourceFromGrid(prev.grid, cell.id);
        return { ...prev, players: updatePlayer(newPlayers, playerIndex, inventory, player), grid: newGrid };
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
  ) {
    if (inventory[type]) {
      inventory[type].count += 1;
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
    const updatedPlayer = {
      ...player,
      inventory,
      energy: Math.max(0, player.energy - 1),
    };
    players[playerIndex] = updatedPlayer;
    return players;
  }

  return { attackPlayerOrMonster, openBarrel, tryExitThroughPortal, collectResourceIfOnTile };
}
