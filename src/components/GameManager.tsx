// GameManager.tsx
"use client";

import React, { useEffect, useCallback } from "react";
import { useGameContext } from "./GameContext";
import Map from "./Map";
import Players from "./Players";
import Inventory from "./Inventory";
import { generateMap } from "../logic/generateMap";
import { useBattleSystem } from "../logic/battleSystem";
import { useArtifactLogic } from "../logic/artifactLogic";
import { handleKeyDown } from "../logic/inputHandler";

type GameManagerProps = {
  inventoryOpen: boolean;
  setInventoryOpen: (open: boolean) => void;
};

export default function GameManager({ inventoryOpen, setInventoryOpen }: GameManagerProps) {
  const { state, setState } = useGameContext();
  const { attackPlayerOrMonster, openBarrel, tryExitThroughPortal, collectResourceIfOnTile } = useBattleSystem();
  const { pickArtifact, loseArtifact, notifyArtifactOwner } = useArtifactLogic();

  const players = state.players;
  const currentPlayerIndex = state.currentPlayerIndex;

  // Все хуки (useEffect, useCallback) вызываем до любых условных return
  useEffect(() => {
    if (state.grid === null && players.length > 0) {
      const newGrid = generateMap(state.mode, players, state.mapWidth, state.mapHeight);
      setState((prev) => ({ ...prev, grid: newGrid }));
    }
  }, [state.grid, players, state.mode, state.mapWidth, state.mapHeight, setState]);

  const onKeyDown = useCallback((e: KeyboardEvent) => {
    handleKeyDown(e, {
      state,
      setState,
      attackPlayerOrMonster,
      openBarrel,
      pickArtifact,
      loseArtifact,
      notifyArtifactOwner,
      tryExitThroughPortal,
      collectResourceIfOnTile,
      inventoryOpen,
      setInventoryOpen
    });
  }, [
    state,
    setState,
    attackPlayerOrMonster,
    openBarrel,
    pickArtifact,
    loseArtifact,
    notifyArtifactOwner,
    tryExitThroughPortal,
    collectResourceIfOnTile,
    inventoryOpen,
    setInventoryOpen
  ]);

  useEffect(() => {
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [onKeyDown]);

  // Теперь после вызова всех хуков делаем условный рендер
  if (players.length === 0 || currentPlayerIndex < 0 || currentPlayerIndex >= players.length) {
    return <div>Загрузка...</div>;
  }

  const activePlayer = players[currentPlayerIndex];

  const passTurn = () => {
    if (!activePlayer.abilities.canPassTurn) return;
    setState((prev) => {
      const nextIndex = (prev.currentPlayerIndex + 1) % prev.players.length;
      return { ...prev, currentPlayerIndex: nextIndex };
    });
  };

  return (
    <div>
      <p>
        {activePlayer.name}: HP={activePlayer.health}, Energy={activePlayer.energy}/{activePlayer.maxEnergy}, Attack=
        {activePlayer.attack}, Defense={activePlayer.defense}, Level={activePlayer.level}
      </p>
      <button onClick={passTurn}>Передать ход</button>
      {state.grid && (
        <Map
          grid={state.grid}
          playerPositions={players.map((p) => p.position)}
          visionRange={activePlayer.visionRange}
          mapWidth={state.mapWidth}
          mapHeight={state.mapHeight}
          activePlayerIndex={currentPlayerIndex}
        />
      )}
      <Players players={players} activePlayerId={activePlayer.id} />
      {inventoryOpen && (
        <Inventory
          items={activePlayer.inventory}
          onUseItem={(type: string) => {
            console.log(`Using item ${type}`);
          }}
        />
      )}
    </div>
  );
}
