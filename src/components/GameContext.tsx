
//GameContext.tsx


"use client";

import React, { createContext, useContext, useReducer, useEffect } from "react";
import { gameReducer } from "../logic/reducer";
import { GameMode, GameState } from "../logic/types";
import { Action } from "../logic/actions";
import { generateMap } from "../logic/generateMap";
import initialPlayers from "../logic/initialPlayers";

type GameContextValue = {
  state: GameState;
  dispatch: React.Dispatch<Action>;
};

type GameProviderProps = {
  instanceId: string;
  children: React.ReactNode;
};

const initialState: GameState = {
  mode: GameMode.ONE_VS_ONE,
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
  inBattle: false,
  battleParticipants: null,
  artifactSelection: null,
};

const GameContext = createContext<GameContextValue | undefined>(undefined);

export function GameProvider({ instanceId, children }: GameProviderProps) {
  const [state, dispatch] = useReducer(gameReducer, { ...initialState, instanceId });

  useEffect(() => {

  
    const generatedGrid = generateMap(state.mode, initialPlayers, state.mapWidth, state.mapHeight);
    

    dispatch({
      type: "INITIALIZE_GAME",
      payload: { mode: state.mode, instanceId, players: initialPlayers },
    });
    dispatch({ type: "SET_GRID", payload: { grid: generatedGrid } });
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
