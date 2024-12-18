// src/components/GameManager.tsx

"use client";

import React, { useEffect, useRef, useMemo, useCallback } from "react";
import { useGameContext } from "./GameContext";
import Map from "./Map";
import Players from "./Players";
import Inventory from "./Inventory";
import BattleScene from "./BattleScene";
import { generateMap } from "../logic/generateMap";
import { useBattleSystem } from "../logic/battleSystem";
import { useResourceSystem } from "../logic/resourceSystem";
import { useArtifactLogic } from "../logic/artifactLogic";
import { handleKeyDown } from "../logic/inputHandler";
import { Action } from "../logic/actions";
import { Entity } from "../logic/types";

export default function GameManager() {
  const { state, dispatch } = useGameContext();

  const { attackPlayerOrMonster, monstersAttackPlayers } = useBattleSystem();
  const { openBarrel, tryExitThroughPortal, collectResourceIfOnTile } = useResourceSystem();
  const { pickArtifact, loseArtifact, notifyArtifactOwner } = useArtifactLogic();

  const players = state.players;
  const currentPlayerIndex = state.currentPlayerIndex;

  const handlersRef = useRef({
    attackPlayerOrMonster,
    openBarrel,
    tryExitThroughPortal,
    collectResourceIfOnTile,
    pickArtifact,
    loseArtifact,
    notifyArtifactOwner,
    inventoryOpen: state.inventoryOpen,
    setInventoryOpen: () => dispatch({ type: 'TOGGLE_INVENTORY' }),
  });

  useEffect(() => {
    handlersRef.current = {
      attackPlayerOrMonster,
      openBarrel,
      tryExitThroughPortal,
      collectResourceIfOnTile,
      pickArtifact,
      loseArtifact,
      notifyArtifactOwner,
      inventoryOpen: state.inventoryOpen,
      setInventoryOpen: () => dispatch({ type: 'TOGGLE_INVENTORY' }),
    };
  }, [
    attackPlayerOrMonster,
    openBarrel,
    tryExitThroughPortal,
    collectResourceIfOnTile,
    pickArtifact,
    loseArtifact,
    notifyArtifactOwner,
    state.inventoryOpen,
    dispatch,
  ]);

  useEffect(() => {
    if (state.turnCycle > 1 && !state.monstersHaveAttacked) {
      monstersAttackPlayers();
      dispatch({ type: "SET_MONSTERS_HAVE_ATTACKED", payload: { monstersHaveAttacked: true } });
    }
  }, [state.turnCycle, state.monstersHaveAttacked, dispatch, monstersAttackPlayers]);

  useEffect(() => {
    if (state.grid.length === 0 && players.length > 0) {
      const newGrid = generateMap(state.mode, players, state.mapWidth, state.mapHeight);
      dispatch({ type: 'SET_GRID', payload: { grid: newGrid } });
    }
  }, [state.grid, players, state.mode, state.mapWidth, state.mapHeight, dispatch]);

  const onKeyDown = useCallback(
    (e: KeyboardEvent) => {
      handleKeyDown(e, { state, dispatch, ...handlersRef.current });
    },
    [state, dispatch]
  );

  useEffect(() => {
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [onKeyDown]);

  const activePlayer = useMemo(
    () =>
      players.length > 0 && currentPlayerIndex >= 0 && currentPlayerIndex < players.length
        ? players[currentPlayerIndex]
        : null,
    [players, currentPlayerIndex]
  );

  const passTurn = useCallback(() => {
    if (!activePlayer?.abilities?.canPassTurn) return;

    dispatch({ type: 'PASS_TURN' });
  }, [activePlayer, dispatch]);

  const memoizedMap = useMemo(
    () =>
      state.grid && activePlayer ? (
        <Map
          grid={state.grid}
          playerPositions={players.map((p) => p.position)}
          visionRange={activePlayer.visionRange}
          mapWidth={state.mapWidth}
          mapHeight={state.mapHeight}
          activePlayerIndex={currentPlayerIndex}
        />
      ) : null,
    [state.grid, players, activePlayer, state.mapWidth, state.mapHeight, currentPlayerIndex]
  );

  const memoizedPlayers = useMemo(
    () =>
      activePlayer ? (
        <Players players={players} activePlayerId={activePlayer.id} />
      ) : null,
    [players, activePlayer]
  );

  const memoizedInventory = useMemo(
    () =>
      state.inventoryOpen && activePlayer?.inventory ? (
        <Inventory
          items={activePlayer.inventory}
          // onUseItem проп убран, так как Inventory теперь обрабатывает dispatch самостоятельно
        />
      ) : null,
    [state.inventoryOpen, activePlayer]
  );

  const onBattleEnd = (result: "attacker-win" | "defender-win", updatedAttacker: Entity) => {
    console.log(`Бой завершен: ${result}, обновленный атакующий:`, updatedAttacker);
    dispatch({ type: 'END_BATTLE', payload: { result, updatedAttacker } });
  };

  return (
    <div>
      {state.inBattle && state.battleParticipants ? (
        <BattleScene
          attacker={state.battleParticipants.attacker}
          defender={state.battleParticipants.defender}
          onBattleEnd={onBattleEnd}
          gridSize={7} // Задаем желаемый размер поля боя
        />
      ) : (
        <>
          {activePlayer && (
            <>
              <p>
                {activePlayer.name}: HP={activePlayer.health}, Energy={activePlayer.energy}/{activePlayer.maxEnergy},
                Attack={activePlayer.attack}, Defense={activePlayer.defense}, Level={activePlayer.level}
              </p>
              <button onClick={passTurn}>Передать ход</button>
            </>
          )}
          {memoizedMap}
          {memoizedPlayers}
          {memoizedInventory}
        </>
      )}
    </div>
  );
}
