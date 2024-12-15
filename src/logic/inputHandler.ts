// inputHandler.ts

import { useGameContext } from "../components/GameContext";

type Handlers = {
  attackPlayerOrMonster: (playerId: number, direction:{dx:number;dy:number}) => void;
  openBarrel: (playerId: number, direction:{dx:number;dy:number}) => void;
  pickArtifact: (playerId: number) => void;
  loseArtifact: (playerId: number) => void;
  notifyArtifactOwner: (playerId: number) => void;
  tryExitThroughPortal: (playerId: number) => void;
  collectResourceIfOnTile: (playerId: number) => void;
  inventoryOpen: boolean;
  setInventoryOpen: (open:boolean)=>void;
};

export function handleKeyDown(
  e: KeyboardEvent,
  { state, setState, attackPlayerOrMonster, openBarrel, pickArtifact, loseArtifact, notifyArtifactOwner, tryExitThroughPortal, collectResourceIfOnTile, inventoryOpen, setInventoryOpen }: {state:any;setState:any}&Handlers
) {
  if (!state.grid || state.players.length === 0) return;
  const playerId = state.players[state.currentPlayerIndex].id;
  const player = state.players.find((p:any)=> p.id===playerId);
  if (!player) return;

  // Инвентарь на "i"
  if (e.key === "i") {
    setInventoryOpen(!inventoryOpen);
    return;
  }

  if (player.energy <= 0) return; // нет энергии - не двигаемся

  let dx = 0, dy = 0;
  if (e.key === "ArrowUp") dy = -1;
  if (e.key === "ArrowDown") dy = 1;
  if (e.key === "ArrowLeft") dx = -1;
  if (e.key === "ArrowRight") dx = 1;

  if ((dx !== 0 || dy !== 0) && state.grid) {
    // Проверяем, можно ли двигаться
    const newX = Math.max(0, Math.min(state.mapWidth-1, player.position.x+dx));
    const newY = Math.max(0, Math.min(state.mapHeight-1, player.position.y+dy));
    if (newX === player.position.x && newY === player.position.y) {
      // Игрок упёрся в край карты
      return;
    }

    // Проверяем тайл, можно ли идти по нему
    const cell = state.grid.find((c:any) => c.x === newX && c.y === newY);
    if (!cell || cell.terrain.includes("river")) {
      // Не можем идти по реке
      return;
    }

    // Двигаемся
    setState((prev:any) => {
      const {players} = prev;
      const pIndex = players.findIndex((p:any)=>p.id===playerId);
      if (pIndex===-1) return prev;

      const updated = {...player, position:{x:newX,y:newY}, energy: Math.max(0, player.energy-1)};
      const newPlayers = [...players];
      newPlayers[pIndex]=updated;
      console.log("Игрок переместился:", updated);
      return {...prev, players:newPlayers};
    });
  }

  if (e.key === " ") {
    // Пробел: взаимодействие с тайлом под игроком
    collectResourceIfOnTile(playerId);
    tryExitThroughPortal(playerId);
  }
}
