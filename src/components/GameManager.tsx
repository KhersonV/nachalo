//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*************************************** src/components/GameManager.tsx ******************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************

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

import { Entity, GameState } from "../logic/types";

export default function GameManager() {
  const { state, dispatch } = useGameContext();

  // Системы
  const { attackPlayerOrMonsterSameCell, monstersAttackPlayers } = useBattleSystem();
  const { openBarrel, tryExitThroughPortal, collectResourceIfOnTile } = useResourceSystem();
  const { pickArtifact, loseArtifact, notifyArtifactOwner } = useArtifactLogic();

  const players = state.players;
  const activePlayer = state.players.find((p) => p.id === state.currentPlayerId) ?? null;

  // Храним все хендлеры в ref, чтобы передавать в handleKeyDown
  const handlersRef = useRef({
    attackPlayerOrMonsterSameCell,
    openBarrel: (playerId: number) => openBarrel(playerId),
    tryExitThroughPortal,
    collectResourceIfOnTile,
    pickArtifact,
    loseArtifact,
    notifyArtifactOwner,
    inventoryOpen: state.inventoryOpen,
    setInventoryOpen: () => dispatch({ type: "TOGGLE_INVENTORY" }),
  });

  // Обновляем ref при изменении зависимостей
  useEffect(() => {
    handlersRef.current = {
      attackPlayerOrMonsterSameCell,
      openBarrel: (playerId: number) => openBarrel(playerId),
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

  // Монстры атакуют (один раз за цикл)
  useEffect(() => {
    if (state.turnCycle > 1 && !state.monstersHaveAttacked) {
      monstersAttackPlayers();
      dispatch({
        type: "SET_MONSTERS_HAVE_ATTACKED",
        payload: { monstersHaveAttacked: true },
      });
    }
  }, [state.turnCycle, state.monstersHaveAttacked, dispatch, monstersAttackPlayers]);

  // Клавиатурные события
  const onKeyDown = useCallback(
    (e: KeyboardEvent) => {
      e.preventDefault();
      handleKeyDown(e, {
        state,
        dispatch,
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
        activePlayerIndex={state.players.findIndex((p) => p.id === activePlayer.id)}
      />
    );
  }, [state.grid, players, activePlayer, state.mapWidth, state.mapHeight]);

  // Мемо списка игроков
  const memoizedPlayers = useMemo(() => {
    if (!activePlayer) return null;
    return <Players players={players} activePlayerId={activePlayer.id} />;
  }, [players, activePlayer]);

  // Мемо инвентаря
  const memoizedInventory = useMemo(() => {
    if (!state.inventoryOpen || !activePlayer) return null;
    return (
      <Inventory
        resources={activePlayer.inventory.resources}
        artifacts={activePlayer.inventory.artifacts}
      />
    );
  }, [state.inventoryOpen, activePlayer]);

  // Определяем cellId для монстра
  function findBattleCellId(
    gameState: GameState,
    participants: { attacker: Entity; defender: Entity }
  ): number {
    const { attacker, defender } = participants;
    const cellWithAttackerMonster = gameState.grid.find(
      (c) => c.monster && c.monster.id === attacker.id
    );
    if (cellWithAttackerMonster) return cellWithAttackerMonster.id;

    const cellWithDefenderMonster = gameState.grid.find(
      (c) => c.monster && c.monster.id === defender.id
    );
    if (cellWithDefenderMonster) return cellWithDefenderMonster.id;

    return -1;
  }

  // Завершаем бой
  const onBattleEnd = useCallback(
    (result: "attacker-win" | "defender-win", payload: { updatedAttacker: Entity; updatedDefender: Entity; cellId: number }) => {
      dispatch({
        type: "END_BATTLE",
        payload: {
          result,
          updatedAttacker: payload.updatedAttacker,
          updatedDefender: payload.updatedDefender,
          cellId: payload.cellId,
        },
      });
    },
    [dispatch]
  );

  // ---- Артефакт ----
  const artifactSelection = state.artifactSelection;
  
  // Локальный ref для "не посылать экшен дважды"
  const artifactTransferRef = useRef(false);

  const handleArtifactTransfer = useCallback(
    (artifactKey: string) => {
      if (!artifactSelection) {
        console.log("GameManager => handleArtifactTransfer called, but no artifactSelection");
        return;
      }
      // Если мы уже кликали – выходим
      if (artifactTransferRef.current) {
        console.log("GameManager => handleArtifactTransfer double-click prevented");
        return;
      }
      // Ставим флажок, чтобы блокировать повтор
      artifactTransferRef.current = true;

      console.log("GameManager => handleArtifactTransfer called!");
      dispatch({
        type: "COMPLETE_ARTIFACT_SELECTION",
        payload: {
          winnerId: artifactSelection.winnerId,
          loserId: artifactSelection.loserId,
          artifactKey,
        },
      });
    },
    [dispatch, artifactSelection]
  );

  // Когда нажимаем "Отмена" – снимаем флажок
  const cancelArtifactTransfer = useCallback(() => {
    artifactTransferRef.current = false;
    dispatch({ type: "CANCEL_ARTIFACT_SELECTION" });
  }, [dispatch]);

  // Если есть artifactSelection – показываем диалог
  if (artifactSelection) {
    return (
      <ArtifactTransferDialog
        artifacts={artifactSelection.artifacts}
        onSelectArtifact={handleArtifactTransfer}
        onCancel={cancelArtifactTransfer}
      />
    );
  }

  // Если идёт бой
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

  // Иначе обычная карта
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
