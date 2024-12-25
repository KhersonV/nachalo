// src/components/GameManager.tsx

import React, { useEffect, useRef, useMemo, useCallback } from "react";
import { useGameContext } from "./GameContext";
import Map from "./Map";
import Players from "./Players";
import Inventory from "./Inventory";
import BattleScene from "./BattleScene";
import ArtifactTransferDialog from "./ArtifactTransferDialog";

import { useBattleSystem } from "../logic/battleSystem";
import { useResourceSystem } from "../logic/resourceSystem";
import { useArtifactLogic } from "../logic/artifactLogic";
import { handleKeyDown } from "../logic/inputHandler";

import { Entity, GameState, PlayerState } from "../logic/types";
import { Action } from "../logic/actions";

export default function GameManager() {
  const { state, dispatch } = useGameContext();

  // Системы для атак, ресурсов, артефактов
  const { attackPlayerOrMonsterSameCell, monstersAttackPlayers } = useBattleSystem();
  const { openBarrel, tryExitThroughPortal, collectResourceIfOnTile } = useResourceSystem();
  const { pickArtifact, loseArtifact, notifyArtifactOwner } = useArtifactLogic();

  const players = state.players;
  const currentPlayerIndex = state.currentPlayerIndex;
  const activePlayer = players.length > 0 ? players[currentPlayerIndex] : null;

  // Ссылка на все обработчики, чтобы удобно передать в handleKeyDown
  const handlersRef = useRef({
    attackPlayerOrMonsterSameCell,
    openBarrel: (playerId: number) => {
      // Логика открытия бочки (без направления, т.к. используем player.position)
      openBarrel(playerId);
    },
    tryExitThroughPortal,
    collectResourceIfOnTile,
    pickArtifact,
    loseArtifact,
    notifyArtifactOwner,
    inventoryOpen: state.inventoryOpen,
    setInventoryOpen: () => dispatch({ type: "TOGGLE_INVENTORY" }),
  });

  // Обновляем handlersRef при изменении зависимостей
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

  // Монстры атакуют один раз за цикл
  useEffect(() => {
    if (state.turnCycle > 1 && !state.monstersHaveAttacked) {
      monstersAttackPlayers();
      dispatch({
        type: "SET_MONSTERS_HAVE_ATTACKED",
        payload: { monstersHaveAttacked: true },
      });
    }
  }, [state.turnCycle, state.monstersHaveAttacked, dispatch, monstersAttackPlayers]);

  // Обработка клавиш
  const onKeyDown = useCallback(
    (e: KeyboardEvent) => {
      e.preventDefault();
      handleKeyDown(e, {
        state,
        dispatch,
        // Передаём все handlers из ref
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

  // Кнопка "Передать ход"
  const passTurn = useCallback(() => {
    if (!activePlayer) return;
    dispatch({ type: "PASS_TURN" });
  }, [activePlayer, dispatch]);

  // Мемоизация карты
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
  }, [
    state.grid,
    players,
    activePlayer,
    state.mapWidth,
    state.mapHeight,
    currentPlayerIndex,
  ]);

  // Мемоизация списка игроков
  const memoizedPlayers = useMemo(() => {
    if (!activePlayer) return null;
    return <Players players={players} activePlayerId={activePlayer.id} />;
  }, [players, activePlayer]);

  // Мемоизация инвентаря (открывается, если state.inventoryOpen === true)
  const memoizedInventory = useMemo(() => {
    if (!state.inventoryOpen || !activePlayer) return null;
    return (
      <Inventory
        resources={activePlayer.inventory.resources}
        artifacts={activePlayer.inventory.artifacts}
      />
    );
  }, [state.inventoryOpen, activePlayer]);

  // Функция определения cellId для монстра, если нужно
  function findBattleCellId(
    gameState: GameState,
    participants: { attacker: Entity; defender: Entity }
  ): number {
    // Если защитник или атакующий - монстр, ищем клетку, где он стоит
    const { attacker, defender } = participants;
    // Поищем монстра, если есть
    const cellWithAttackerMonster = gameState.grid.find(
      (c) => c.monster && c.monster.id === attacker.id
    );
    if (cellWithAttackerMonster) return cellWithAttackerMonster.id;

    const cellWithDefenderMonster = gameState.grid.find(
      (c) => c.monster && c.monster.id === defender.id
    );
    if (cellWithDefenderMonster) return cellWithDefenderMonster.id;

    return -1; // Если оба игроки — монстров нет
  }

  // Колбэк конца боя
  const onBattleEnd = useCallback(
    (result: "attacker-win" | "defender-win", updatedAttacker: Entity, cellId: number) => {
      dispatch({ type: "END_BATTLE", payload: { result, updatedAttacker, cellId } });
    },
    [dispatch]
  );

  // ------------------------------------
  // ЛОГИКА ВЫБОРА АРТЕФАКТА
  // ------------------------------------
  const artifactSelection = state.artifactSelection;

  // Когда пользователь кликает на один из артефактов проигравшего:
  const handleArtifactTransfer = (artifactKey: string) => {
    if (!artifactSelection) return;
    dispatch({
      type: "COMPLETE_ARTIFACT_SELECTION",
      payload: {
        winnerId: artifactSelection.winnerId,
        loserId: artifactSelection.loserId,
        artifactKey,
      },
    });
  };

  // Отмена окна выбора:
  const cancelArtifactTransfer = () => {
    dispatch({ type: "CANCEL_ARTIFACT_SELECTION" });
  };

  // ------------------------------------
  // РЕНДЕР
  // ------------------------------------
  // 1) Если в стейте есть флаг artifactSelection — показываем окно
  if (artifactSelection) {
    return (
      <ArtifactTransferDialog
        artifacts={artifactSelection.artifacts}
        onSelectArtifact={handleArtifactTransfer}
        onCancel={cancelArtifactTransfer}
      />
    );
  }

  // 2) Иначе если идёт бой — показываем BattleScene
  if (state.inBattle && state.battleParticipants) {
    const { attacker, defender } = state.battleParticipants;
    return (
      <BattleScene
        attacker={attacker}
        defender={defender}
        cellId={findBattleCellId(state, state.battleParticipants)}
        onBattleEnd={onBattleEnd}
        gridSize={7}
      />
    );
  }

  // 3) Если нет выбора артефакта и нет боя, рендерим обычную карту и т.д.
  return (
    <div>
      {activePlayer && (
        <>
          <p>
            {activePlayer.name}: 
            HP={activePlayer.health}, 
            Energy={activePlayer.energy}/{activePlayer.maxEnergy}, 
            Attack={activePlayer.attack}, 
            Defense={activePlayer.defense}, 
            Level={activePlayer.level}
          </p>
          <button onClick={passTurn}>Передать ход</button>
        </>
      )}

      {memoizedMap}
      {memoizedPlayers}
      {memoizedInventory}
    </div>
  );
}
