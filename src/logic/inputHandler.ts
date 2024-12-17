// src/logic/inputHandler.ts

import { Action } from "./actions";
import { PlayerState, GameState } from "./types";

type Handlers = {
  attackPlayerOrMonster: (playerId: number, direction: { dx: number; dy: number }) => void;
  openBarrel: (playerId: number, direction: { dx: number; dy: number }) => void;
  pickArtifact: (playerId: number) => void;
  loseArtifact: (playerId: number) => void;
  notifyArtifactOwner: () => void;
  tryExitThroughPortal: (playerId: number) => void;
  collectResourceIfOnTile: (playerId: number) => void;
  inventoryOpen: boolean;
  setInventoryOpen: () => void;
};

export function handleKeyDown(
  e: KeyboardEvent,
  {
    state,
    dispatch,
    attackPlayerOrMonster,
    openBarrel,
    pickArtifact,
    loseArtifact,
    notifyArtifactOwner,
    tryExitThroughPortal,
    collectResourceIfOnTile,
    inventoryOpen,
    setInventoryOpen,
  }: { state: GameState; dispatch: React.Dispatch<Action> } & Handlers
) {
  if (!state.grid || state.players.length === 0) return;
  const playerId = state.players[state.currentPlayerIndex].id;
  const player = state.players.find((p: PlayerState) => p.id === playerId);
  if (!player) return;

  // Инвентарь на "i"
  if (e.key === "i" || e.key === "I" || e.key === "ш" || e.key === "Ш") {
    dispatch({ type: 'TOGGLE_INVENTORY' });
    return;
  }

  if (player.energy <= 0) return; // нет энергии - не двигаемся

  let dx = 0,
    dy = 0;
  if (e.key === "ArrowUp") dy = -1;
  if (e.key === "ArrowDown") dy = 1;
  if (e.key === "ArrowLeft") dx = -1;
  if (e.key === "ArrowRight") dx = 1;

  if ((dx !== 0 || dy !== 0) && state.grid) {
    // Проверяем, можно ли двигаться
    const newX = Math.max(0, Math.min(state.mapWidth - 1, player.position.x + dx));
    const newY = Math.max(0, Math.min(state.mapHeight - 1, player.position.y + dy));
    if (newX === player.position.x && newY === player.position.y) {
      // Игрок упёрся в край карты
      return;
    }

    // Проверяем тайл, можно ли идти по нему
    const cell = state.grid.find(c => c.x === newX && c.y === newY);
    if (!cell || cell.terrain.includes("river")) {
      // Не можем идти по реке
      return;
    }

    dispatch({ type: 'MOVE_PLAYER', payload: { playerId, newPosition: { x: newX, y: newY } } });
  }

  if (e.key === " ") {
    // Пробел: взаимодействие с тайлом под игроком
    const currentCell = state.grid.find(c => c.x === player.position.x && c.y === player.position.y);
    if (currentCell?.resource) {
      dispatch({ 
        type: 'COLLECT_RESOURCE', 
        payload: { 
          playerId, 
          resourceType: currentCell.resource.type, 
          cellId: currentCell.id 
        } 
      });
    }
    if (currentCell?.isPortal) {
      dispatch({ type: 'TRY_EXIT_PORTAL', payload: { playerId } });
    }
  }
}
