//resourceSystem.ts

import { useGameContext } from "../components/GameContext";
import { createRandomMonster } from "../logic/monsterData";
import { resources, getResource } from "../components/resources/ResourceData";

export function useResourceSystem() {
  const { state, setState } = useGameContext();

  /**
   * Обрабатывает взаимодействие с бочкой.
   */
  const openBarrel = (playerId: number, direction: { dx: number; dy: number }) => {
    setState((prev) => {
      if (!prev.grid) return prev;

      const playerIndex = prev.players.findIndex((p) => p.id === playerId);
      if (playerIndex === -1) return prev;

      const player = prev.players[playerIndex];
      const targetCell = prev.grid.find(
        (c) => c.x === player.position.x + direction.dx && c.y === player.position.y + direction.dy
      );

      if (!targetCell || !targetCell.resource || targetCell.resource.type !== "barrbel") {
        console.log("Бочка отсутствует на указанной клетке.");
        return prev;
      }

      console.log("Открываем бочку");

      const random = Math.random();
      const updatedPlayers = [...prev.players];

      // 5% шанс выпадения артефакта
      if (random < 0.05) {
        console.log("Из бочки выпал артефакт!");
        const inventory = { ...player.inventory };
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
        updatedPlayers[playerIndex] = { ...player, inventory };
        const updatedGrid = prev.grid.map((cell) =>
          cell.id === targetCell.id ? { ...cell, resource: null } : cell
        );
        return { ...prev, players: updatedPlayers, grid: updatedGrid };
      }

      // 35% шанс появления монстра
      if (random < 0.4) {
        const randomMonster = createRandomMonster();
        console.log(`Из бочки выпал монстр: ${randomMonster.name}`);
        const newGrid = prev.grid.map((cell) =>
          cell.id === targetCell.id ? { ...cell, monster: randomMonster, resource: null } : cell
        );
        return { ...prev, grid: newGrid };
      }

      // 60% шанс появления ресурса
      console.log("Из бочки выпал ресурс!");
      const possibleResources = Object.keys(resources).filter((key) => key !== "barrbel");
      const randomResourceKey =
        possibleResources[Math.floor(Math.random() * possibleResources.length)];
      const selectedResource = getResource(randomResourceKey);

      if (!selectedResource) {
        console.error(`Не удалось найти ресурс для ключа: ${randomResourceKey}`);
        return prev;
      }

      const inventory = { ...player.inventory };
      if (inventory[randomResourceKey]) {
        inventory[randomResourceKey].count += 1;
      } else {
        inventory[randomResourceKey] = {
          count: 1,
          description: selectedResource.description,
          image: selectedResource.image["ground"],
        };
      }
      updatedPlayers[playerIndex] = { ...player, inventory };
      const updatedGrid = prev.grid.map((cell) =>
        cell.id === targetCell.id ? { ...cell, resource: null } : cell
      );
      return { ...prev, players: updatedPlayers, grid: updatedGrid };
    });
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

      if (!cell || !cell.isPortal) {
        console.log("Портал отсутствует на клетке.");
        return prev;
      }

      if (prev.artifactOwner === playerId) {
        console.log(`Игрок ${player.name} успешно покидает портал с артефактом!`);
        // Дополнительная логика завершения игры может быть добавлена здесь
      } else {
        console.log("Игрок не может выйти через портал без артефакта.");
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

      if (!cell || !cell.resource) {
        return prev;
      }

      const inventory = JSON.parse(JSON.stringify(player.inventory));
      const resource = cell.resource;

      // Увеличиваем количество ресурса в инвентаре
      if (inventory[resource.type]) {
        inventory[resource.type].count += 1;
      } else {
        inventory[resource.type] = {
          count: 1,
          image: resource.image["ground"] || "/default-resource.webp",
          description: resource.description,
        };
      }

      // Удаляем ресурс с клетки
      const newGrid = prev.grid.map((c) =>
        c.id === cell.id ? { ...c, resource: null } : c
      );

      const updatedPlayers = [...prev.players];
      updatedPlayers[playerIndex] = { ...player, inventory };

      return { ...prev, players: updatedPlayers, grid: newGrid };
    });
  };

  return {
    openBarrel,
    tryExitThroughPortal,
    collectResourceIfOnTile,
  };
}
