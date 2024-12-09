"use client";

import React, { useEffect } from "react";
import { useGameContext } from "./GameContext";
import { resources, ResourceType } from "./resources/ResourceData";
import Map from "./Map";
import Players from "./Players";
import Inventory from "./Inventory";

export default function GameManager({ inventoryOpen }: { inventoryOpen: boolean }) {
  const { state, setState } = useGameContext();

  // Генерация карты на клиенте
  useEffect(() => {
    if (state.grid !== null) return; // уже сгенерировано
    const terrains = ["water", "ground", "mountain", "forest", "ice"];
    const { mapWidth, mapHeight } = state;

    const generateResource = (terrain: string): ResourceType | null => {
      const availableResources = Object.values(resources).filter((res) =>
        res.terrains.includes(terrain)
      );
      if (availableResources.length === 0) return null;
      return availableResources[Math.floor(Math.random() * availableResources.length)];
    };

    const generateGrid = (width: number, height: number) => {
      const total = width * height;
      return Array.from({ length: total }, (_, id) => {
        const x = id % width;
        const y = Math.floor(id / width);
        const terrain = terrains[Math.floor(Math.random() * terrains.length)];
        const resource = Math.random() < 0.15 ? generateResource(terrain) : null;
        return { id, x, y, terrain, resource };
      });
    };

    const newGrid = generateGrid(mapWidth, mapHeight);
    setState((prev) => ({ ...prev, grid: newGrid }));
  }, [state.grid, state.mapWidth, state.mapHeight, setState]);

  // Пока один игрок – активный игрок всегда первый
  const activePlayer = state.players[0];

  // В дальнейшем здесь можно разделить логику ввода для разных режимов
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (state.grid === null) return;
      // PvE: Управляем только одним игроком
      // Позже: Если PvP, можно будет распределять управление по игрокам или обрабатывать сетевые события
      if (e.key === "ArrowUp") {
        movePlayer(0, 0, -1);
      } else if (e.key === "ArrowDown") {
        movePlayer(0, 0, 1);
      } else if (e.key === "ArrowLeft") {
        movePlayer(0, -1, 0);
      } else if (e.key === "ArrowRight") {
        movePlayer(0, 1, 0);
      } else if (e.key === " ") {
        collectResource(0);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [state.grid]);

  function movePlayer(playerId: number, dx: number, dy: number) {
    setState((prev) => {
      const { players, mapWidth, mapHeight } = prev;
      const playerIndex = players.findIndex((p) => p.id === playerId);
      if (playerIndex === -1) return prev;

      const player = players[playerIndex];
      if (player.energy <= 0) return prev; // нет энергии – нет движения

      const newX = Math.max(0, Math.min(mapWidth - 1, player.position.x + dx));
      const newY = Math.max(0, Math.min(mapHeight - 1, player.position.y + dy));

      const updatedPlayer = {
        ...player,
        position: { x: newX, y: newY },
        energy: Math.max(0, player.energy - 1),
      };

      const newPlayers = [...players];
      newPlayers[playerIndex] = updatedPlayer;
      return { ...prev, players: newPlayers };
    });
  }

  function collectResource(playerId: number) {
    setState((prev) => {
      const { players, grid } = prev;
      if (!grid) return prev;

      const pIndex = players.findIndex((p) => p.id === playerId);
      if (pIndex === -1) return prev;

      const player = players[pIndex];
      if (player.energy <= 0) return prev; // нет энергии

      const cellIndex = grid.findIndex((c) => c.x === player.position.x && c.y === player.position.y);
      if (cellIndex === -1) return prev;
      const cell = grid[cellIndex];
      if (!cell.resource) return prev;

      // Добавляем в инвентарь игрока
      const { type, image, description } = cell.resource;
      const newPlayers = [...players];
      const currentInv = { ...player.inventory };
      if (currentInv[type]) {
        currentInv[type].count += 1;
      } else {
        currentInv[type] = { count: 1, image, description };
      }

      newPlayers[pIndex] = {
        ...player,
        energy: Math.max(0, player.energy - 1),
        inventory: currentInv,
      };

      const newGrid = grid.map((c) => (c.id === cell.id ? { ...c, resource: null } : c));

      return { ...prev, players: newPlayers, grid: newGrid };
    });
  }

  // Использование предметов при клике в инвентаре
  function useItem(playerId: number, itemType: string) {
    // Аналогично, изменяем state игроков
  }

  if (state.grid === null) {
    return <div>Loading...</div>;
  }

  return (
    <div>
      <p>Игрок: X={activePlayer.position.x}, Y={activePlayer.position.y}, HP={activePlayer.health}, Energy={activePlayer.energy}/{activePlayer.maxEnergy}, Attack={activePlayer.attack}, Defense={activePlayer.defense}, Level={activePlayer.level}</p>
      <Map
        grid={state.grid}
        playerPositions={state.players.map((p) => p.position)}
        visionRange={activePlayer.visionRange}
        mapWidth={state.mapWidth}
        mapHeight={state.mapHeight}
      />
      <Players players={state.players} activePlayerId={activePlayer.id} />
      {inventoryOpen && (
        <Inventory
          items={activePlayer.inventory}
          onUseItem={(type) => useItem(activePlayer.id, type)}
        />
      )}
    </div>
  );
}
