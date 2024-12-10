"use client";

import React, { createContext, useContext, useState, useEffect } from "react";

export type GameMode = "PVE"|"1v1"|"3v3"|"5v5";

export type PlayerState = {
  id: number;
  name: string;
  position: { x: number; y: number };
  energy: number;
  maxEnergy: number;
  level: number;
  visionRange: number;
  health: number;
  maxHealth: number;
  attack: number;
  defense: number;
  inventory: Record<string, { count: number; image: string; description: string }>;
};

export type ResourceType = {
  type: string;
  image: string;
  description: string;
  terrains: string[];
};

export type Cell = {
  id: number;
  x: number;
  y: number;
  terrain: string;
  resource: ResourceType | null;
  isBarrel?: boolean;
  isPortal?: boolean;
  isMonster?: boolean;
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
  currentPlayerIndex: number; // индекс активного игрока
};

type GameContextValue = {
  state: GameState;
  setState: React.Dispatch<React.SetStateAction<GameState>>;
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
    // Имитируем получение данных о режиме и игроках из бэкенда
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
          level: 10,
          visionRange: 5,
          health: 100,
          maxHealth: 100,
          attack: 10,
          defense: 5,
          inventory: {}
        },
        {
          id: 1,
          name: "Player2",
          position: { x: 19, y: 19 },
          energy: 100,
          maxEnergy: 100,
          level: 10,
          visionRange: 5,
          health: 100,
          maxHealth: 100,
          attack: 10,
          defense: 5,
          inventory: {}
        }
      ]
    }));
  }, [instanceId]);

  return <GameContext.Provider value={{ state, setState }}>{children}</GameContext.Provider>;
}

export function useGameContext() {
  const context = useContext(GameContext);
  if (!context) {
    throw new Error("useGameContext must be used within a GameProvider");
  }
  return context;
}
