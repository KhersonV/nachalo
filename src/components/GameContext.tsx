"use client";

import React, { createContext, useState, useContext } from "react";

export type PlayerState = {
  id: number;
  position: { x: number; y: number };
  energy: number;
  maxEnergy: number;
  level: number;
  visionRange: number;
  health: number;
  attack: number;
  defense: number;
  inventory: Record<string, { count: number; image: string; description: string }>;
};

export type GameMode = "PVE" | "1v1" | "3v3" | "5v5";

export type Cell = {
  id: number;
  x: number;
  y: number;
  terrain: string;
  resource: { type: string; image: string; description: string } | null;
};

type GameState = {
  mode: GameMode;
  players: PlayerState[];
  grid: Cell[] | null;
  mapWidth: number;
  mapHeight: number;
};

type GameContextValue = {
  state: GameState;
  setState: React.Dispatch<React.SetStateAction<GameState>>;
};

const GameContext = createContext<GameContextValue | null>(null);

export function GameProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<GameState>({
    mode: "PVE", // по умолчанию PVE, потом можно менять
    players: [
      {
        id: 0,
        position: { x: 0, y: 0 },
        energy: 100,
        maxEnergy: 100,
        level: 1,
        visionRange: 5,
        health: 100,
        attack: 10,
        defense: 5,
        inventory: {},
      },
    ],
    grid: null,
    mapWidth: 20,
    mapHeight: 20,
  });

  return <GameContext.Provider value={{ state, setState }}>{children}</GameContext.Provider>;
}

export function useGameContext() {
  const context = useContext(GameContext);
  if (!context) {
    throw new Error("useGameContext must be used within a GameProvider");
  }
  return context;
}
