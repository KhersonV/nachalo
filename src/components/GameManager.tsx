// src/components/GameManager.tsx

import React, { useEffect, useRef, useMemo, useCallback } from "react";
import { useGameContext } from "./GameContext";
import Map from "./Map";
import Players from "./Players";
import Inventory from "./Inventory";
import BattleScene from "./BattleScene";
import { useBattleSystem } from "../logic/battleSystem";
import { useResourceSystem } from "../logic/resourceSystem";
import { useArtifactLogic } from "../logic/artifactLogic";
import { handleKeyDown } from "../logic/inputHandler";
import { Entity, GameState } from "../logic/types";

export default function GameManager() {
  const { state, dispatch } = useGameContext();

  // Получаем новую функцию:
  const { attackPlayerOrMonsterSameCell, monstersAttackPlayers } = useBattleSystem();
  const { openBarrel, tryExitThroughPortal, collectResourceIfOnTile } = useResourceSystem();
  const { pickArtifact, loseArtifact, notifyArtifactOwner } = useArtifactLogic();

  const players = state.players;
  const currentPlayerIndex = state.currentPlayerIndex;

  // Обратите внимание: теперь мы передаём в handlersRef
  // именно attackPlayerOrMonsterSameCell (без направления).
  const handlersRef = useRef({
    attackPlayerOrMonsterSameCell,
    openBarrel: (playerId: number) => {
      // Реализуйте открытие бочки без направления, если нужно.
      openBarrel(playerId); 
      // Или полностью перепишите openBarrel так, чтобы не принимала dx/dy.
    },
    tryExitThroughPortal,
    collectResourceIfOnTile,
    pickArtifact,
    loseArtifact,
    notifyArtifactOwner,
    inventoryOpen: state.inventoryOpen,
    setInventoryOpen: () => dispatch({ type: "TOGGLE_INVENTORY" }),
  });

  useEffect(() => {
    handlersRef.current = {
      attackPlayerOrMonsterSameCell,
      openBarrel: (playerId: number) => {
        openBarrel(playerId);
      },
      tryExitThroughPortal,
      collectResourceIfOnTile,
      pickArtifact,
      loseArtifact,
      notifyArtifactOwner,
      inventoryOpen: state.inventoryOpen,
      setInventoryOpen: () => dispatch({ type: "TOGGLE_INVENTORY" }),
    };
  }, [
    attackPlayerOrMonsterSameCell,
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
    // Монстры ходят один раз за цикл
    if (state.turnCycle > 1 && !state.monstersHaveAttacked) {
      monstersAttackPlayers();
      dispatch({
        type: "SET_MONSTERS_HAVE_ATTACKED",
        payload: { monstersHaveAttacked: true },
      });
    }
  }, [state.turnCycle, state.monstersHaveAttacked, dispatch, monstersAttackPlayers]);

  const onKeyDown = useCallback(
    (e: KeyboardEvent) => {
      e.preventDefault();
      handleKeyDown(e, {
        state,
        dispatch,
        // Разворачиваем handlersRef.current,
        ...handlersRef.current,
      });
    },
    [state, dispatch]
  );

  useEffect(() => {
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [onKeyDown]);

  // Текущий игрок
  const activePlayer = useMemo(() => {
    if (players.length > 0 && currentPlayerIndex >= 0 && currentPlayerIndex < players.length) {
      return players[currentPlayerIndex];
    }
    return null;
  }, [players, currentPlayerIndex]);

  const passTurn = useCallback(() => {
    if (!activePlayer) return;
    dispatch({ type: "PASS_TURN" });
  }, [activePlayer, dispatch]);

  // Карта
  const memoizedMap = useMemo(() => {
    if (!state.grid || !activePlayer) return null;
    return (
      <Map
        grid={state.grid}
        playerPositions={players.map((p) => p.position)}
        visionRange={activePlayer.visionRange}
        mapWidth={state.mapWidth}
        mapHeight={state.mapHeight}
        activePlayerIndex={currentPlayerIndex}
      />
    );
  }, [state.grid, players, activePlayer, state.mapWidth, state.mapHeight, currentPlayerIndex]);

  // Отображение игроков (списком)
  const memoizedPlayers = useMemo(() => {
    if (!activePlayer) return null;
    return <Players players={players} activePlayerId={activePlayer.id} />;
  }, [players, activePlayer]);

  // Инвентарь
  const memoizedInventory = useMemo(() => {
    if (!state.inventoryOpen || !activePlayer?.inventory) return null;
    return <Inventory items={activePlayer.inventory} />;
  }, [state.inventoryOpen, activePlayer]);

  // Колбэк конца боя
  const onBattleEnd = useCallback(
    (result: "attacker-win" | "defender-win", updatedAttacker: Entity, cellId: number) => {
      console.log(
        `Бой завершен: ${result}, обновленный атакующий:`,
        updatedAttacker,
        `cellId=${cellId}`
      );
      dispatch({ type: "END_BATTLE", payload: { result, updatedAttacker, cellId } });
    },
    [dispatch]
  );

  // Функция поиска cellId (если надо)
  function findBattleCellId(
    state: GameState,
    battleParticipants: { attacker: Entity; defender: Entity }
  ): number {
    // Если атакующий - монстр
    if (!("level" in battleParticipants.attacker)) {
      const cell = state.grid.find(
        (c) => c.monster && c.monster.id === battleParticipants.attacker.id
      );
      if (cell) return cell.id;
    }

    // Если защитник - монстр
    if (!("level" in battleParticipants.defender)) {
      const cell = state.grid.find(
        (c) => c.monster && c.monster.id === battleParticipants.defender.id
      );
      if (cell) return cell.id;
    }

    return -1; // не найден
  }

  return (
    <div>
      {/* Если идёт бой, показываем только BattleScene */}
      {state.inBattle && state.battleParticipants ? (
        <BattleScene
          attacker={state.battleParticipants.attacker}
          defender={state.battleParticipants.defender}
          cellId={findBattleCellId(state, state.battleParticipants)}
          onBattleEnd={onBattleEnd}
          gridSize={7}
        />
      ) : (
        // ИНАЧЕ (если боя нет) - рендерим основную карту и т.д.
        <>
          {activePlayer && (
            <>
              <p>
                {activePlayer.name}: HP={activePlayer.health}, Energy={activePlayer.energy}/
                {activePlayer.maxEnergy}, Attack={activePlayer.attack}, Defense={
                  activePlayer.defense
                }, Level={activePlayer.level}
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