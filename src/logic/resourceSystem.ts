//resourceSystem.ts

import { useGameContext } from "../components/GameContext";
import { createRandomMonster } from "../logic/monsterData";
import { resources, getResource } from "../components/resources/ResourceData";

export function useResourceSystem() {
  const { state, setState } = useGameContext();

  /**
   * Обрабатывает взаимодействие с бочкой.
   * @param playerId - ID игрока, который взаимодействует с бочкой.
   * @param direction - Направление (dx, dy), в котором игрок ищет бочку.
   */
  const openBarrel = (playerId: number, direction: { dx: number; dy: number }) => {
    setState((prev) => {
      if (!prev.grid) return prev; // Если сетка отсутствует, ничего не делаем.

      // Находим игрока по его ID.
      const playerIndex = prev.players.findIndex((p) => p.id === playerId);
      if (playerIndex === -1) return prev; // Если игрока нет, возвращаем текущее состояние.

      const player = prev.players[playerIndex];

      // Определяем клетку, где должна быть бочка.
      const targetCell = prev.grid.find(
        (c) => c.x === player.position.x + direction.dx && c.y === player.position.y + direction.dy
      );

      if (!targetCell || !targetCell.resource || targetCell.resource.type !== "barrbel") {
        console.log("Бочка отсутствует на указанной клетке.");
        return prev; // Если бочки нет, ничего не делаем.
      }

      console.log("Открываем бочку");

      const random = Math.random(); // Генерируем случайное число для определения результата взаимодействия.
      const updatedPlayers = [...prev.players]; // Копируем массив игроков для обновления.

      // 5% шанс выпадения артефакта.
      if (random < 0.05) {
        console.log("Из бочки выпал артефакт!");
        const inventory = { ...player.inventory }; // Копируем инвентарь игрока.

        // Добавляем артефакт в инвентарь.
        if (inventory["artifact"]) {
          inventory["artifact"].count += 1;
        } else {
          inventory["artifact"] = {
            count: 1,
            description: "Могущественный артефакт. Дает бонусы к атрибутам.",
            image: "/resources/artifact.webp",
            bonus: { attack: 5, defense: 5, energy: 10 },
          };
        }

        updatedPlayers[playerIndex] = { ...player, inventory }; // Обновляем игрока.
        const updatedGrid = prev.grid.map((cell) =>
          cell.id === targetCell.id ? { ...cell, resource: null } : cell
        ); // Убираем ресурс из клетки.

        return { ...prev, players: updatedPlayers, grid: updatedGrid }; // Возвращаем обновлённое состояние.
      }

      // 35% шанс появления монстра.
      if (random < 0.4) {
        const randomMonster = createRandomMonster(); // Генерируем случайного монстра.
        console.log(`Из бочки выпал монстр: ${randomMonster.name}`);

        // Обновляем клетку, добавляя монстра.
        const newGrid = prev.grid.map((cell) =>
          cell.id === targetCell.id ? { ...cell, monster: randomMonster, resource: null } : cell
        );

        return { ...prev, grid: newGrid }; // Возвращаем обновлённое состояние сетки.
      }

      // 60% шанс появления ресурса.
      console.log("Из бочки выпал ресурс!");

      // Получаем случайный ресурс, исключая "barrbel".
      const possibleResources = Object.keys(resources).filter((key) => key !== "barrbel");
      const randomResourceKey =
        possibleResources[Math.floor(Math.random() * possibleResources.length)];
      const selectedResource = getResource(randomResourceKey);

      if (!selectedResource) {
        console.error(`Не удалось найти ресурс для ключа: ${randomResourceKey}`);
        return prev; // Если ресурс не найден, возвращаем текущее состояние.
      }

      const inventory = { ...player.inventory }; // Копируем инвентарь игрока.

      // Добавляем ресурс в инвентарь.
      if (inventory[randomResourceKey]) {
        inventory[randomResourceKey].count += 1;
      } else {
        inventory[randomResourceKey] = {
          count: 1,
          description: selectedResource.description,
          image: selectedResource.image["ground"],
        };
      }

      updatedPlayers[playerIndex] = { ...player, inventory }; // Обновляем игрока.
      const updatedGrid = prev.grid.map((cell) =>
        cell.id === targetCell.id ? { ...cell, resource: null } : cell
      ); // Убираем ресурс из клетки.

      return { ...prev, players: updatedPlayers, grid: updatedGrid }; // Возвращаем обновлённое состояние.
    });
  };

  /**
   * Обрабатывает попытку выхода через портал.
   * @param playerId - ID игрока, пытающегося выйти через портал.
   */
  const tryExitThroughPortal = (playerId: number) => {
    setState((prev) => {
      if (!prev.grid) return prev; // Если сетка отсутствует, ничего не делаем.

      const player = prev.players.find((p) => p.id === playerId);
      if (!player) return prev; // Если игрока нет, возвращаем текущее состояние.

      // Определяем клетку, где находится игрок.
      const cell = prev.grid.find(
        (c) => c.x === player.position.x && c.y === player.position.y
      );

      if (!cell || !cell.isPortal) {
        console.log("Портал отсутствует на клетке.");
        return prev; // Если портала нет, ничего не делаем.
      }

      if (prev.artifactOwner === playerId) {
        console.log(`Игрок ${player.name} успешно покидает портал с артефактом!`);
        // Здесь можно добавить логику завершения игры.
      } else {
        console.log("Игрок не может выйти через портал без артефакта.");
      }

      return prev;
    });
  };

  /**
   * Обрабатывает сбор ресурса с клетки.
   * @param playerId - ID игрока, собирающего ресурс.
   */
  const collectResourceIfOnTile = (playerId: number) => {
    setState((prev) => {
      if (!prev.grid) return prev; // Если сетка отсутствует, ничего не делаем.

      const playerIndex = prev.players.findIndex((p) => p.id === playerId);
      if (playerIndex === -1) return prev; // Если игрока нет, возвращаем текущее состояние.

      const player = prev.players[playerIndex];

      // Определяем клетку, где находится игрок.
      const cell = prev.grid.find(
        (c) => c.x === player.position.x && c.y === player.position.y
      );

      if (!cell || !cell.resource) {
        return prev; // Если ресурса нет, ничего не делаем.
      }

      const inventory = JSON.parse(JSON.stringify(player.inventory)); // Глубокое копирование инвентаря.
      const resource = cell.resource;

      // Добавляем ресурс в инвентарь.
      if (inventory[resource.type]) {
        inventory[resource.type].count += 1;
      } else {
        inventory[resource.type] = {
          count: 1,
          image: resource.image["ground"] || "/default-resource.webp",
          description: resource.description,
        };
      }

      // Убираем ресурс с клетки.
      const newGrid = prev.grid.map((c) =>
        c.id === cell.id ? { ...c, resource: null } : c
      );

      const updatedPlayers = [...prev.players];
      updatedPlayers[playerIndex] = { ...player, inventory };

      return { ...prev, players: updatedPlayers, grid: newGrid }; // Возвращаем обновлённое состояние.
    });
  };

  return {
    openBarrel,
    tryExitThroughPortal,
    collectResourceIfOnTile,
  };
}
