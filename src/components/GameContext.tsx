"use client";

import React, { createContext, useContext, useState, useEffect } from "react";
import { MonsterState } from "../logic/monsterData";
import { resources } from "./resources/ResourceData";

export type GameMode = "PVE"|"1v1"|"3v3"|"5v5";

export type PlayerState = {
  id: number;
  name: string;
  position: { x: number; y: number };
  energy: number;
  maxEnergy: number;
  level: number;
  expirience: number;
  max_expirience:number;
  visionRange: number;
  health: number;
  maxHealth: number;
  attack: number;
  defense: number;
  image: string;
  inventory: Record<string, { count: number; image: string; description: string }>;
};

export type ResourceType = {
  type: string;
  image: string;
  description: string;
  terrains: string[];
  effect: number;
};

export type Cell = {
  id: number;
  x: number;
  y: number;
  terrain: string;
  resource: ResourceType | null;
  isBarrel?: boolean;
  isPortal?: boolean;
  monster?: MonsterState; 
};

type GameState = {
  mode: GameMode;
  players: PlayerState[];
  grid: Cell[] | null;
  mapWidth: number;
  mapHeight: number;
  artifactOwner: number | null;
  portalPosition: {x: number; y: number} | null;
  instanceId: string;
  currentPlayerIndex: number;
};

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
  });

  useEffect(() => {
    // Имитация загрузки данных
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
          visionRange: 3,
          health: 100,
          maxHealth: 100,
          attack: 10,
          defense: 5,
          image: "player-1.webp",
          inventory: {}
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
          inventory: {}
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
    
        // Обновляем состояние
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
