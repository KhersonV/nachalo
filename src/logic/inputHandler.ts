
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//************************************************* src/logic/inputHandler.ts *************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************

import { Action } from "./actions";
import { PlayerState, GameState } from "./types";

type Handlers = {
  // УДАЛЯЕМ:
  // attackPlayerOrMonster: (playerId: number, direction: { dx: number; dy: number }) => void;
  // openBarrel: (playerId: number, direction: { dx: number; dy: number }) => void;

  // ОСТАЛЬНОЕ оставляем:
  pickArtifact: (playerId: number) => void;
  loseArtifact: (playerId: number) => void;
  notifyArtifactOwner: () => void;
  tryExitThroughPortal: (playerId: number) => void;
  collectResourceIfOnTile: (playerId: number) => void;
  inventoryOpen: boolean;
  setInventoryOpen: () => void;

  // НОВОЕ:
  attackPlayerOrMonsterSameCell: (playerId: number) => void;
  openBarrel: (playerId: number) => void; // Если нужно открывать бочку без направления
};

export function handleKeyDown(
  e: KeyboardEvent,
  {
    state,
    dispatch,
    pickArtifact,
    loseArtifact,
    notifyArtifactOwner,
    tryExitThroughPortal,
    collectResourceIfOnTile,
    inventoryOpen,
    setInventoryOpen,
    attackPlayerOrMonsterSameCell,
    openBarrel,
  }: { state: GameState; dispatch: React.Dispatch<Action> } & Handlers
) {
  if (!state.grid || state.players.length === 0) return;
  const playerId = state.players[state.currentPlayerIndex].id;
  const player = state.players.find((p: PlayerState) => p.id === playerId);
  if (!player) return;

  // Инвентарь на "i"
  if (["i", "I", "ш", "Ш"].includes(e.key)) {
    dispatch({ type: "TOGGLE_INVENTORY" });
    return;
  }

  // Если у игрока нет энергии, то он не может двигаться.
  if (player.energy <= 0) return;

  let dx = 0,
    dy = 0;
  if (e.key === "ArrowUp") dy = -1;
  if (e.key === "ArrowDown") dy = 1;
  if (e.key === "ArrowLeft") dx = -1;
  if (e.key === "ArrowRight") dx = 1;

  // Движение
  if ((dx !== 0 || dy !== 0) && state.grid) {
    const newX = Math.max(0, Math.min(state.mapWidth - 1, player.position.x + dx));
    const newY = Math.max(0, Math.min(state.mapHeight - 1, player.position.y + dy));

    // Если упёрлись в край карты
    if (newX === player.position.x && newY === player.position.y) {
      return;
    }

    // Проверяем, проходима ли клетка
    const cell = state.grid.find((c) => c.x === newX && c.y === newY);
    if (!cell || cell.terrain.includes("river")) {
      // Не можем идти по реке
      return;
    }

    // Двигаем игрока
    dispatch({
      type: "MOVE_PLAYER",
      payload: { playerId, newPosition: { x: newX, y: newY } },
    });
  }

  // Пробел: взаимодействие (сбор ресурса, портал и т.п.)
  if (e.key === " ") {
    const currentCell = state.grid.find(
      (c) => c.x === player.position.x && c.y === player.position.y
    );
    if (currentCell?.resource) {

      if (currentCell.resource.type === "barrbel") {
        openBarrel(playerId);
      } else {

      dispatch({
        type: "COLLECT_RESOURCE",
        payload: {
          playerId,
          resourceType: currentCell.resource.type,
          cellId: currentCell.id,
        },
      });
    }
    if (currentCell?.isPortal) {
      dispatch({ type: "TRY_EXIT_PORTAL", payload: { playerId } });
    }
  }}

  // Клавиша "A": атака, НО БЕЗ НАПРАВЛЕНИЯ
  if (["A", "a", "Ф", "ф"].includes(e.key)) {
    attackPlayerOrMonsterSameCell(playerId);
  }
}
