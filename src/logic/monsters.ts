import { Cell, PlayerState } from "../components/GameContext";
  
// Функция агрессивных монстров: по окончании хода атакуют игрока, если видят
export function aggressiveMonstersAttack(state: {players: PlayerState[], grid: Cell[]|null, artifactOwner:number|null, mode:string}): {newState: typeof state, instanceFinished:boolean} {
  if(!state.grid) return {newState:state, instanceFinished:false};
  let newPlayers = [...state.players];

  // Пробегаем по всем монстрам
  for (const cell of state.grid) {
    if(cell.monster && cell.monster.type === 'aggressive') {
      // Проверяем, есть ли игрок в радиусе vision
      // Вариант: берем первого попавшегося игрока для простоты
      for(let i=0; i<newPlayers.length; i++) {
        const player = newPlayers[i];
        const dist = Math.abs(player.position.x - cell.x) + Math.abs(player.position.y - cell.y);
        if(dist <= cell.monster.vision) {
          // Монстр атакует игрока
          const damage = Math.max(0, cell.monster.attack - player.defense);
          const newHp = player.health - damage;
          newPlayers[i] = {...player, health: newHp};
          console.log(`${cell.monster.name} атакует ${player.name} на ${damage} урона. HP у игрока: ${newHp}`);
          if(newHp <= 0) {
            console.log(`Игрок ${player.name} убит!`);
            // Можно считать конец инстанса или продолжать
          }
        }
      }
    }
  }

  return {newState: {...state, players:newPlayers}, instanceFinished:false};
}
