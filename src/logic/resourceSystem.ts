// src/logic/resourceSystem.ts

import { useGameContext } from "../components/GameContext";
import { createRandomMonster } from "./allData";
import { resources, getResource } from "./allData";
import { Action } from "./actions";
import { GameState, PlayerState, Cell, MonsterState } from "./types";

export function useResourceSystem() {
  const { state, dispatch } = useGameContext();

  const openBarrel = (playerId: number, direction: { dx: number; dy: number }) => {
    if (!state.grid) {
      console.error("Grid is not initialized.");
      return;
    }

    const player: PlayerState | undefined = state.players.find(p => p.id === playerId);
    if (!player) {
      console.error(`Player with id ${playerId} not found.`);
      return;
    }

    const targetX = player.position.x + direction.dx;
    const targetY = player.position.y + direction.dy;

    const targetCell: Cell | undefined = state.grid.find(c => c.x === targetX && c.y === targetY);
    if (!targetCell || !targetCell.resource || targetCell.resource.type !== "barrbel") {
      console.log("Бочка отсутствует на указанной клетке.");
      return;
    }

    console.log("Открываем бочку");
    const random = Math.random();

    if (random < 0.05) {
      console.log("Из бочки выпал артефакт!");
      dispatch({
        type: 'ADD_ITEM',
        payload: {
          playerId,
          itemType: 'artifact',
          description: "Могущественный артефакт. Дает бонусы к атрибутам.",
          image: "/resources/artifact.webp",
          bonus: { attack: 5, defense: 5, energy: 10 },
        },
      });
      dispatch({ type: 'REMOVE_RESOURCE', payload: { cellId: targetCell.id } });
    } else if (random < 0.4) {
      const randomMonster: MonsterState = createRandomMonster();
      console.log(`Из бочки выпал монстр: ${randomMonster.name}`);
      dispatch({
        type: 'ADD_MONSTER',
        payload: { cellId: targetCell.id, monster: randomMonster },
      });
      dispatch({ type: 'REMOVE_RESOURCE', payload: { cellId: targetCell.id } });
    } else {
      console.log("Из бочки выпал ресурс!");
      const possibleResources: string[] = Object.keys(resources).filter((key) => key !== "barrbel");
      const randomResourceKey: string = possibleResources[Math.floor(Math.random() * possibleResources.length)];
      const selectedResource = getResource(randomResourceKey);

      if (!selectedResource) {
        console.error(`Не удалось найти ресурс для ключа: ${randomResourceKey}`);
        return;
      }

      dispatch({
        type: 'ADD_ITEM',
        payload: {
          playerId,
          itemType: randomResourceKey,
          description: selectedResource.description,
          image: selectedResource.image["ground"],
        },
      });
      dispatch({ type: 'REMOVE_RESOURCE', payload: { cellId: targetCell.id } });
    }
  };

  const tryExitThroughPortal = (playerId: number) => {
    if (!state.grid) {
      console.error("Grid is not initialized.");
      return;
    }

    const player: PlayerState | undefined = state.players.find(p => p.id === playerId);
    if (!player) {
      console.error(`Player with id ${playerId} not found.`);
      return;
    }

    const cell: Cell | undefined = state.grid.find(c => c.x === player.position.x && c.y === player.position.y);
    if (!cell || !cell.isPortal) {
      console.log("Портал отсутствует на клетке.");
      return;
    }

    if (state.artifactOwner === playerId) {
      console.log(`Игрок ${player.name} успешно покидает портал с артефактом!`);
      // Добавьте действие для завершения инстанса
      // Например:
      // dispatch({ type: 'FINALIZE_INSTANCE', payload: { instanceId: state.instanceId, players: state.players } });
    } else {
      console.log("Игрок не может выйти через портал без артефакта.");
    }
  };

  const collectResourceIfOnTile = (playerId: number) => {
    if (!state.grid) {
      console.error("Grid is not initialized.");
      return;
    }
  
    const player: PlayerState | undefined = state.players.find(p => p.id === playerId);
    if (!player) {
      console.error(`Player with id ${playerId} not found.`);
      return;
    }
  
    const cell: Cell | undefined = state.grid.find(c => c.x === player.position.x && c.y === player.position.y);
    if (!cell || !cell.resource) {
      console.log("На клетке нет ресурса для сбора.");
      return;
    }
  
    dispatch({ type: 'COLLECT_RESOURCE', payload: { playerId, resourceType: cell.resource.type, cellId: cell.id } });
  };

  return {
    openBarrel,
    tryExitThroughPortal,
    collectResourceIfOnTile,
  };
}
