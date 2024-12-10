import { GameContextValue } from "../components/GameContext";

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

  // Открытие/закрытие инвентаря на клавишу "i"
  if (e.key === "i" || e.key === "ш" || e.key === "I" || e.key === "Ш" ) {
    setInventoryOpen(!inventoryOpen);
    return;
  }

  if (player.energy <= 0) return; // нет энергии

  let dx = 0, dy = 0;
  if (e.key === "ArrowUp") dy = -1;
  if (e.key === "ArrowDown") dy = 1;
  if (e.key === "ArrowLeft") dx = -1;
  if (e.key === "ArrowRight") dx = 1;

  if (dx !== 0 || dy !== 0) {
    // Движение
    setState((prev:any) => {
      const {players, mapWidth, mapHeight} = prev;
      const pIndex = players.findIndex((p:any)=>p.id===playerId);
      if (pIndex===-1) return prev;

      const newX = Math.max(0, Math.min(mapWidth-1, player.position.x+dx));
      const newY = Math.max(0, Math.min(mapHeight-1, player.position.y+dy));
      // Тратим 1 энергию
      const updated = {...player, position:{x:newX,y:newY}, energy: Math.max(0, player.energy-1)};
      const newPlayers = [...players];
      newPlayers[pIndex]=updated;
      return {...prev, players:newPlayers};
    });
  }

  if (e.key === " ") {
    // Пробел: взаимодействие с текущим тайлом
    // Сбор ресурса, открытие бочки, попытка выхода через портал
    collectResourceIfOnTile(playerId);
    tryExitThroughPortal(playerId);
    // Если на тайле бочка - открыть бочку (упрощаем, бочка открывается автоматически)
    // Если хотим направление атаки или открытия бочки - можно дополнить логику.
  }
}
