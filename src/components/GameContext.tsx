"use client";

import React, { createContext, useContext, useState, useEffect } from "react";
import { resources } from "./resources/ResourceData";
import { GameState, PlayerAbilities } from "../logic/types";

// Определяем тип для контекста игры, который включает состояние игры, функцию для обновления состояния,
// а также метод для применения эффекта ресурса.
type GameContextValue = {
  state: GameState;
  setState: React.Dispatch<React.SetStateAction<GameState>>;
  applyResourceEffect: (playerId: number, resourceType: string) => void;
};

// Определяем тип свойств для GameProvider.
type GameProviderProps = {
  instanceId: string; // Уникальный идентификатор экземпляра игры.
  children: React.ReactNode; // Дочерние компоненты.
};

// Создаём контекст игры с начальным значением null.
const GameContext = createContext<GameContextValue | null>(null);

// Компонент-провайдер для управления состоянием игры.
export function GameProvider({ instanceId, children }: GameProviderProps) {
  // Состояние игры. Изначально задаётся пустое значение, которое затем инициализируется.
  const [state, setState] = useState<GameState>({
    mode: "1v1", // Режим игры.
    players: [], // Список игроков.
    grid: null, // Игровое поле.
    mapWidth: 20, // Ширина карты.
    mapHeight: 20, // Высота карты.
    artifactOwner: null, // Владелец артефакта.
    portalPosition: null, // Позиция портала.
    instanceId, // Уникальный идентификатор экземпляра игры.
    currentPlayerIndex: 0, // Индекс текущего игрока.
    turnCycle: 1, // Номер текущего игрового цикла.
  });

  // Хук для инициализации игроков при монтировании компонента.
  useEffect(() => {
    // Способности, которые изначально доступны всем игрокам.
    const defaultAbilities: PlayerAbilities = {
      canMove: true,
      canAttack: true,
      canCollectResources: true,
      canUseItems: true,
      canInteractWithObjects: true,
      canPassTurn: true,
      canPickArtifact: true,
      canLoseArtifact: true,
    };

    // Устанавливаем начальное состояние игроков.
    setState((prev) => ({
      ...prev,
      mode: "1v1", // Устанавливаем режим игры.
      players: [
        {
          id: 0,
          name: "Player1", // Имя игрока.
          position: { x: 0, y: 0 }, // Начальная позиция.
          energy: 100, // Текущая энергия.
          maxEnergy: 100, // Максимальная энергия.
          level: 1, // Уровень.
          expirience: 0, // Текущий опыт.
          max_expirience: 500, // Максимальный опыт для повышения уровня.
          visionRange: 5, // Радиус видимости.
          health: 100, // Текущее здоровье.
          maxHealth: 100, // Максимальное здоровье.
          attack: 10, // Уровень атаки.
          defense: 5, // Уровень защиты.
          image: "player-1.webp", // Изображение игрока.
          inventory: {}, // Инвентарь.
          abilities: { ...defaultAbilities }, // Способности игрока.
        },
        {
          id: 1,
          name: "Player2",
          position: { x: 19, y: 19 },
          energy: 100,
          maxEnergy: 100,
          level: 1,
          expirience: 0,
          max_expirience: 500,
          visionRange: 3,
          health: 100,
          maxHealth: 100,
          attack: 10,
          defense: 5,
          image: "player-2.webp",
          inventory: {},
          abilities: { ...defaultAbilities },
        },
      ],
    }));
  }, [instanceId]);

  // Функция для применения эффекта ресурса к указанному игроку.
  function applyResourceEffect(playerId: number, resourceType: string) {
    setState((prev) => {
      // Ищем игрока по его ID.
      const playerIndex = prev.players.findIndex((p) => p.id === playerId);
      if (playerIndex === -1) return prev; // Если игрок не найден, возвращаем текущее состояние.

      const player = prev.players[playerIndex];
      const resource = resources[resourceType]; // Получаем ресурс по типу.
      if (!resource || !player.inventory[resourceType]?.count) return prev; // Если ресурса нет, ничего не делаем.

      // Копируем игрока для обновления.
      let updatedPlayer = { ...player };

      // Применяем эффекты ресурса в зависимости от его типа.
      switch (resourceType) {
        case "food":
          // Увеличиваем здоровье.
          updatedPlayer.health = Math.min(
            updatedPlayer.maxHealth,
            player.health + resource.effect
          );
          break;
        case "water":
          // Увеличиваем энергию.
          updatedPlayer.energy = Math.min(
            updatedPlayer.maxEnergy,
            player.energy + resource.effect
          );
          break;
        case "stone":
          // Увеличиваем защиту.
          updatedPlayer.defense += resource.effect;
          break;
        case "iron":
          // Увеличиваем атаку.
          updatedPlayer.attack += resource.effect;
          break;
        case "wood":
          // Увеличиваем опыт.
          updatedPlayer.expirience = Math.min(
            updatedPlayer.max_expirience,
            player.expirience + resource.effect
          );
          break;
        default:
          break;
      }

      // Уменьшаем количество ресурса в инвентаре.
      const updatedInventory = { ...player.inventory };
      updatedInventory[resourceType].count -= 1;
      if (updatedInventory[resourceType].count <= 0) {
        delete updatedInventory[resourceType]; // Удаляем ресурс из инвентаря, если его больше нет.
      }

      updatedPlayer.inventory = updatedInventory;

      // Обновляем список игроков.
      const updatedPlayers = [...prev.players];
      updatedPlayers[playerIndex] = updatedPlayer;

      return { ...prev, players: updatedPlayers }; // Возвращаем обновлённое состояние.
    });
  }

  // Возвращаем провайдер контекста с текущим состоянием и методами.
  return (
    <GameContext.Provider value={{ state, setState, applyResourceEffect }}>
      {children}
    </GameContext.Provider>
  );
}

// Хук для использования контекста игры.
export function useGameContext() {
  const context = useContext(GameContext);
  if (!context) {
    throw new Error("useGameContext must be used within a GameProvider");
  }
  return context;
}
