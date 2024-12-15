//GameManager.tsx

import React, { useEffect, useRef, useMemo, useCallback } from "react";
import { useGameContext } from "./GameContext";
import Map from "./Map";
import Players from "./Players";
import Inventory from "./Inventory";
import { generateMap } from "../logic/generateMap";
import { useBattleSystem } from "../logic/battleSystem";
import { useResourceSystem } from "../logic/resourceSystem";
import { useArtifactLogic } from "../logic/artifactLogic";
import { handleKeyDown } from "../logic/inputHandler";

// Определение типа для пропсов компонента GameManager
type GameManagerProps = {
  inventoryOpen: boolean; // Отвечает за состояние инвентаря (открыт/закрыт)
  setInventoryOpen: (open: boolean) => void; // Функция для изменения состояния инвентаря
};

export default function GameManager({ inventoryOpen, setInventoryOpen }: GameManagerProps) {
  // Используем контекст игры для получения текущего состояния и функции обновления состояния
  const { state, setState } = useGameContext();

  // Получение логики атаки, системы ресурсов и артефактов
  const { attackPlayerOrMonster, monstersAttackPlayers } = useBattleSystem();
  const { openBarrel, tryExitThroughPortal, collectResourceIfOnTile } = useResourceSystem();
  const { pickArtifact, loseArtifact, notifyArtifactOwner } = useArtifactLogic();

  // Извлекаем игроков и индекс текущего игрока из состояния
  const players = state.players;
  const currentPlayerIndex = state.currentPlayerIndex;

  // Храним обработчики действий в рефе, чтобы избежать лишних зависимостей в useCallback
  const handlersRef = useRef({
    attackPlayerOrMonster,
    openBarrel,
    tryExitThroughPortal,
    collectResourceIfOnTile,
    pickArtifact,
    loseArtifact,
    notifyArtifactOwner,
    inventoryOpen,
    setInventoryOpen,
  });

  // Обновляем handlersRef, если обработчики или состояние инвентаря изменились
  useEffect(() => {
    handlersRef.current = {
      attackPlayerOrMonster,
      openBarrel,
      tryExitThroughPortal,
      collectResourceIfOnTile,
      pickArtifact,
      loseArtifact,
      notifyArtifactOwner,
      inventoryOpen,
      setInventoryOpen,
    };
  }, [
    attackPlayerOrMonster,
    openBarrel,
    tryExitThroughPortal,
    collectResourceIfOnTile,
    pickArtifact,
    loseArtifact,
    notifyArtifactOwner,
    inventoryOpen,
    setInventoryOpen,
  ]);

  // Генерация карты, если grid ещё не существует
  useEffect(() => {
    if (state.grid === null && players.length > 0) {
      const newGrid = generateMap(state.mode, players, state.mapWidth, state.mapHeight);
      setState((prev) => ({ ...prev, grid: newGrid }));
    }
  }, [state.grid, players, state.mode, state.mapWidth, state.mapHeight, setState]);

  // Лог текущего состояния игроков при их изменении
  useEffect(() => {
    console.log("Текущее состояние игроков:", state.players);
  }, [state.players]);

  // Обработчик событий клавиатуры
  const onKeyDown = useCallback(
    (e: KeyboardEvent) => {
      handleKeyDown(e, { ...handlersRef.current, state, setState });
    },
    [state, setState]
  );

  // Добавление и удаление обработчиков клавиатуры
  useEffect(() => {
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [onKeyDown]);

  // Определение текущего активного игрока
  const activePlayer = useMemo(
    () =>
      players.length > 0 && currentPlayerIndex >= 0 && currentPlayerIndex < players.length
        ? players[currentPlayerIndex]
        : null,
    [players, currentPlayerIndex]
  );

  // Функция передачи хода
  const passTurn = useCallback(() => {
    if (!activePlayer?.abilities?.canPassTurn) return;

    setState((prev) => {
      const nextIndex = (prev.currentPlayerIndex + 1) % prev.players.length;
      const isEndOfTurn = nextIndex === 0;

      if (isEndOfTurn) {
        console.log(`Круг ${prev.turnCycle} завершен, монстры атакуют игроков.`);
        monstersAttackPlayers();
      }

      return {
        ...prev,
        currentPlayerIndex: nextIndex,
        turnCycle: isEndOfTurn ? prev.turnCycle + 1 : prev.turnCycle,
      };
    });
  }, [activePlayer, monstersAttackPlayers, setState]);

  // Мемоизация карты
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

  // Мемоизация компонента игроков
  const memoizedPlayers = useMemo(
    () =>
      activePlayer ? (
        <Players players={players} activePlayerId={activePlayer.id} />
      ) : null,
    [players, activePlayer]
  );

  // Мемоизация инвентаря
  const memoizedInventory = useMemo(
    () =>
      inventoryOpen && activePlayer?.inventory ? (
        <Inventory
          items={activePlayer.inventory}
          onUseItem={(type: string) => {
            console.log(`Using item ${type}`);
          }}
        />
      ) : null,
    [inventoryOpen, activePlayer?.inventory]
  );

  // Рендер компонента
  return (
    <div>
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
    </div>
  );
}
