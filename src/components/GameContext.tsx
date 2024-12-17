"use client";

import React, { createContext, useContext, useReducer, useEffect } from "react";
import { gameReducer } from "../logic/reducer";
import { GameState, PlayerState } from "../logic/types";
import { Action } from "../logic/actions";
import { generateMap } from "../logic/generateMap";
import { aggressiveMonstersAttack } from "../logic/monsters";
import { checkForDuplicateMonsters } from "../logic/utils";

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
  grid: [],
  mapWidth: 20,
  mapHeight: 20,
  artifactOwner: null,
  portalPosition: null,
  instanceId: "",
  currentPlayerIndex: 0,
  turnCycle: 1,
  inventoryOpen: false,
  monstersHaveAttacked: false,
  
};

const GameContext = createContext<GameContextValue | undefined>(undefined);

export function GameProvider({ instanceId, children }: GameProviderProps) {
  const [state, dispatch] = useReducer(gameReducer, { ...initialState, instanceId });

  useEffect(() => {
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
        speed: 3,           
        maneuverability: 2, 
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
        speed: 3,            
        maneuverability: 2, 
        image: "player-2.webp",
        inventory: {},
        abilities: { ...defaultAbilities },
      },
    ];

    const generatedGrid = generateMap(state.mode, initialPlayers, state.mapWidth, state.mapHeight);
    checkForDuplicateMonsters(generatedGrid);

    dispatch({
      type: "INITIALIZE_GAME",
      payload: { mode: state.mode, instanceId, players: initialPlayers },
    });
    dispatch({ type: "SET_GRID", payload: { grid: generatedGrid } });
  }, [instanceId]);

  useEffect(() => {
    if (!state.monstersHaveAttacked) {
      dispatch({ type: "SET_MONSTERS_HAVE_ATTACKED", payload: { monstersHaveAttacked: false } });
    }

    if (state.turnCycle > 1 && !state.monstersHaveAttacked) {
      aggressiveMonstersAttack(state, dispatch);
      dispatch({ type: "SET_MONSTERS_HAVE_ATTACKED", payload: { monstersHaveAttacked: true } });
    }
  }, [state.turnCycle, state.monstersHaveAttacked, dispatch]);

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
