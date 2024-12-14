// src/components/GameContext.tsx
"use client";

import React, { createContext, useContext, useState, useEffect } from "react";
import { resources } from "./resources/ResourceData";
import { GameState, PlayerAbilities } from "../logic/types";

type GameContextValue = {
  state: GameState;
  setState: React.Dispatch<React.SetStateAction<GameState>>;
  applyResourceEffect: (playerId: number, resourceType: string) => void;
};

type GameProviderProps = {
  instanceId: string;
  children: React.ReactNode;
};

const GameContext = createContext<GameContextValue | null>(null);

export function GameProvider({ instanceId, children }: GameProviderProps) {
  const [state, setState] = useState<GameState>({
    mode: "1v1",
    players: [],
    grid: null,
    mapWidth: 20,
    mapHeight: 20,
    artifactOwner: null,
    portalPosition: null,
    instanceId,
    currentPlayerIndex: 0,
    turnCycle: 1,
  });

  useEffect(() => {
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

    setState(prev => ({
      ...prev,
      mode: "1v1",
      players: [
        {
          id: 0,
          name: "Player1",
          position: { x: 0, y: 0 },
          energy: 100,
          maxEnergy: 100,
          level: 1,
          expirience: 0,
          max_expirience: 500,
          visionRange: 5,
          health: 100,
          maxHealth: 100,
          attack: 10,
          defense: 5,
          image: "player-1.webp",
          inventory: {},
          abilities: { ...defaultAbilities },
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
        }
      ]
    }));
  }, [instanceId]);

  function applyResourceEffect(playerId: number, resourceType: string) {
    setState((prev) => {
      const playerIndex = prev.players.findIndex((p) => p.id === playerId);
      if (playerIndex === -1) return prev;

      const player = prev.players[playerIndex];
      const resource = resources[resourceType];
      if (!resource || !player.inventory[resourceType]?.count) return prev;

      let updatedPlayer = { ...player };

      // Применяем эффект ресурса
      switch (resourceType) {
        case "food":
          updatedPlayer.health = Math.min(updatedPlayer.maxHealth, player.health + resource.effect);
          break;
        case "water":
          updatedPlayer.energy = Math.min(updatedPlayer.maxEnergy, player.energy + resource.effect);
          break;
        case "stone":
          updatedPlayer.defense += resource.effect;
          break;
        case "iron":
          updatedPlayer.attack += resource.effect;
          break;
        case "wood":
          updatedPlayer.expirience = Math.min(
            updatedPlayer.max_expirience,
            player.expirience + resource.effect
          );
          break;
        default:
          break;
      }

      // Уменьшаем количество ресурса в инвентаре
      const updatedInventory = { ...player.inventory };
      updatedInventory[resourceType].count -= 1;
      if (updatedInventory[resourceType].count <= 0) {
        delete updatedInventory[resourceType];
      }

      updatedPlayer.inventory = updatedInventory;

      const updatedPlayers = [...prev.players];
      updatedPlayers[playerIndex] = updatedPlayer;

      return { ...prev, players: updatedPlayers };
    });
  }

  return <GameContext.Provider value={{ state, setState, applyResourceEffect}}>{children}</GameContext.Provider>;
}

export function useGameContext() {
  const context = useContext(GameContext);
  if (!context) {
    throw new Error("useGameContext must be used within a GameProvider");
  }
  return context;
}
