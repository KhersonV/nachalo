//progressionSystem.ts
import { PlayerState } from "./types";

export function finalizeInstance(instanceId: string, players: PlayerState[]) {
  console.log(`Инстанс ${instanceId} завершён. Начисляем награды игрокам.`);
  players.forEach(player => {
    console.log(`Игрок ${player.name} получает опыт и валюту.`);
  });
  // Будущая логика: обновление БД, уровней, валюты
}
