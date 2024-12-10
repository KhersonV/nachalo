import { useGameContext } from "../components/GameContext";

export function useBattleSystem() {
  const { state, setState } = useGameContext();

  function attackPlayerOrMonster(playerId: number, direction:{dx:number;dy:number}) {
    console.log("Attack logic here");
  }

  function openBarrel(playerId: number, direction:{dx:number;dy:number}) {
    console.log("Open barrel logic");
  }

  function tryExitThroughPortal(playerId: number) {
    setState(prev => {
      const player = prev.players.find(p=>p.id===playerId);
      if (!player || !prev.grid) return prev;
      const cell = prev.grid.find(c=>c.x===player.position.x && c.y===player.position.y);
      if (!cell || !cell.isPortal) return prev;
      // Проверяем артефакт
      if (prev.artifactOwner === playerId) {
        console.log("Игрок вышел через портал с артефактом! Инстанс завершен.");
        // Здесь логика завершения инстанса, начисление наград
      }
      return prev;
    });
  }

  function collectResourceIfOnTile(playerId: number) {
    setState(prev => {
      if(!prev.grid) return prev;
      const pIndex = prev.players.findIndex(p=>p.id===playerId);
      if(pIndex===-1) return prev;
      const player = prev.players[pIndex];
      const cell = prev.grid.find(c=>c.x===player.position.x && c.y===player.position.y);
      if (!cell) return prev;

      // Если есть ресурс - добавить в инвентарь и убрать ресурс
      if (cell.resource) {
        const newPlayers = [...prev.players];
        const inventory = {...player.inventory};
        const r = cell.resource;
        if(inventory[r.type]) {
          inventory[r.type].count += 1;
        } else {
          inventory[r.type] = { count:1, image:r.image, description:r.description };
        }
        newPlayers[pIndex] = {...player, inventory, energy: Math.max(0, player.energy-1)};
        const newGrid = prev.grid.map(c=> c.id===cell.id ? {...c, resource:null}: c);
        return {...prev, players:newPlayers, grid:newGrid};
      }

      // Если есть бочка - открыть бочку
      if (cell.isBarrel) {
        console.log("Открываем бочку - может выпасть ресурс, монстр или артефакт");
        // Упростим: бочка даёт food
        const newPlayers = [...prev.players];
        const inventory = {...player.inventory};
        if(inventory['food']) {
          inventory['food'].count += 1;
        } else {
          inventory['food'] = { count:1, image:"/food.webp", description:"Еда для выживания."};
        }
        newPlayers[pIndex]={...player, inventory, energy: Math.max(0, player.energy-1)};
        const newGrid = prev.grid.map(c=> c.id===cell.id ? {...c, isBarrel:false}: c);
        return {...prev, players:newPlayers, grid:newGrid};
      }

      return prev;
    });
  }

  return { attackPlayerOrMonster, openBarrel, tryExitThroughPortal, collectResourceIfOnTile };
}
