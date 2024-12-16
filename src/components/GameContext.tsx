// src/components/GameContext.tsx

"use client";

import React, { createContext, useContext, useReducer, useEffect } from "react";
import { gameReducer } from "../logic/reducer";
import { GameState, PlayerState } from "../logic/types";
import { Action } from "../logic/actions";

type GameContextValue = {
  state: GameState;
  dispatch: React.Dispatch<Action>;
};

type GameProviderProps = {
  instanceId: string;
  children: React.ReactNode;
};

const initialState: GameState = {
  mode: "1v1",
  players: [],
  grid: null,
  mapWidth: 20,
  mapHeight: 20,
  artifactOwner: null,
  portalPosition: null,
  instanceId: "",
  currentPlayerIndex: 0,
  turnCycle: 1,
  inventoryOpen: false, // Инициализируем инвентарь как закрытый
};

const GameContext = createContext<GameContextValue | undefined>(undefined);

export function GameProvider({ instanceId, children }: GameProviderProps) {
  const [state, dispatch] = useReducer(gameReducer, { ...initialState, instanceId });

  useEffect(() => {
    // Инициализация начальных игроков и других параметров
    const defaultAbilities = {
      canMove: true,
      canAttack: true,
      canCollectResources: true,
      canUseItems: true,
      canInteractWithObjects: true,
      canPassTurn: true,
      canPickArtifact: true,
      canLoseArtifact: true,
    };

    const initialPlayers: PlayerState[] = [
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
      },
    ];

    dispatch({ type: 'INITIALIZE_GAME', payload: { mode: "1v1", instanceId, players: initialPlayers } });
  }, [instanceId]);

  return (
    <GameContext.Provider value={{ state, dispatch }}>
      {children}
    </GameContext.Provider>
  );
}

export function useGameContext() {
  const context = useContext(GameContext);
  if (!context) {
    throw new Error("useGameContext must be used within a GameProvider");
  }
  return context;
}
