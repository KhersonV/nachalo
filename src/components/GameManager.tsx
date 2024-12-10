"use client";

import React, { useEffect } from "react";
import { useGameContext } from "./GameContext";
import Map from "./Map";
import Players from "./Players";
import Inventory from "./Inventory";
import { generateMap } from "../logic/generateMap";
import { handleKeyDown } from "../logic/inputHandler";
import { useBattleSystem } from "../logic/battleSystem";
import { useArtifactLogic } from "../logic/artifactLogic";
import { aggressiveMonstersAttack } from "../logic/monsters";
import { finalizeInstance } from "../logic/progressionSystem";

type GameManagerProps = {
  inventoryOpen: boolean;
  setInventoryOpen: (open: boolean) => void;
};

export default function GameManager({ inventoryOpen, setInventoryOpen }: GameManagerProps) {
  const { state, setState } = useGameContext();
  const { attackPlayerOrMonster, openBarrel, tryExitThroughPortal, collectResourceIfOnTile } = useBattleSystem();
  const { pickArtifact, loseArtifact, notifyArtifactOwner } = useArtifactLogic();

  const activePlayer = state.players[state.currentPlayerIndex];

  useEffect(() => {
    if (state.grid === null && state.players.length > 0) {
      const newGrid = generateMap(state.mode, state.players, state.mapWidth, state.mapHeight);
      setState(prev => ({ ...prev, grid: newGrid }));
    }
  }, [state.grid, state.players, state.mode, state.mapWidth, state.mapHeight, setState]);

  useEffect(() => {
    const keyHandler = (e: KeyboardEvent) => {
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
    };
    window.addEventListener("keydown", keyHandler);
    return () => window.removeEventListener("keydown", keyHandler);
  }, [state, setState, attackPlayerOrMonster, openBarrel, pickArtifact, loseArtifact, notifyArtifactOwner, tryExitThroughPortal, collectResourceIfOnTile, inventoryOpen, setInventoryOpen]);

  if (state.grid === null) {
    return <div>Loading...</div>;
  }

  const endTurn = () => {
    // Передача хода следующему игроку
    setState((prev) => {
      const nextIndex = (prev.currentPlayerIndex + 1) % prev.players.length;
      return { ...prev, currentPlayerIndex: nextIndex };
    });

    // Агрессивные монстры атакуют после смены хода
    setTimeout(() => {
      setState(prev => {
        const result = aggressiveMonstersAttack(prev);
        // Если инстанс должен закончиться - вызываем finalizeInstance
        // Пока просто логируем
        if (result.instanceFinished) {
          finalizeInstance(prev.instanceId, prev.players);
        }
        return {
            ...prev, // Сохраняем остальные свойства состояния
            ...result.newState, // Обновляем свойства из результата атаки монстров
          } as typeof prev;;
      });
    }, 100);
  };

  return (
    <div>
      <p>{activePlayer.name}: X={activePlayer.position.x}, Y={activePlayer.position.y}, HP={activePlayer.health}, Energy={activePlayer.energy}/{activePlayer.maxEnergy}, Attack={activePlayer.attack}, Defense={activePlayer.defense}, Level={activePlayer.level}</p>
      <button onClick={endTurn}>Сменить ход</button>
      <Map
        grid={state.grid}
        playerPositions={state.players.map((p) => p.position)}
        visionRange={activePlayer.visionRange}
        mapWidth={state.mapWidth}
        mapHeight={state.mapHeight}
        activePlayerIndex={state.currentPlayerIndex}
      />
      <Players players={state.players} activePlayerId={activePlayer.id} />
      {inventoryOpen && (
        <Inventory
          items={activePlayer.inventory}
          onUseItem={(type: string) => {
            console.log(`Используем предмет ${type}`);
          }}
        />
      )}
    </div>
  );
}
