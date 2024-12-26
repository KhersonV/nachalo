
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//************************************************* src/logic/battleSystem.ts *************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************

import { useGameContext } from "../components/GameContext";
import { Entity, PlayerState, GameState } from "./types";
import { useCallback } from "react";

function isPlayer(entity: Entity): entity is PlayerState {
  return "inventory" in entity;
}

export function useBattleSystem() {
  const { state, dispatch } = useGameContext();

  // Оставляем монстрам возможность атаковать игроков (если нужно)
  const monstersAttackPlayers = useCallback(() => {
    // Ваш код, который проверяет агрессивных монстров и т.д.
    // ...
  }, [state, dispatch]);

  // --- Единственная функция атаки: игрок атакует тех, кто на одной клетке ---
  const attackPlayerOrMonsterSameCell = useCallback((playerId: number) => {
    // 1) Находим атакующего игрока
    const attacker = state.players.find((p) => p.id === playerId);
    if (!attacker) return;

    // 2) Ищем среди игроков — другого игрока, который на той же клетке
    const targetPlayer = state.players.find(
      (p) =>
        p.id !== attacker.id &&
        p.position.x === attacker.position.x &&
        p.position.y === attacker.position.y
    );

    // 3) Ищем среди grid — монстра на той же клетке
    const targetCell = state.grid.find(
      (c) => c.x === attacker.position.x && c.y === attacker.position.y
    );
    const targetMonster = targetCell?.monster;

    // 4) Если нашли игрока, инициируем PvP
    if (targetPlayer) {
      console.log(
        `Игрок ${attacker.name} атакует игрока ${targetPlayer.name} на одной клетке!`
      );
      dispatch({
        type: "START_BATTLE",
        payload: {
          attacker,
          defender: targetPlayer,
          cellId: -1, // если хотим, можем вычислить cellId, но для PvP это не обязательно
        },
      });
      return;
    }

    // 5) Если нашли монстра, инициируем бой с монстром
    if (targetMonster) {
      console.log(
        `Игрок ${attacker.name} атакует монстра ${targetMonster.name} на одной клетке!`
      );
      dispatch({
        type: "START_BATTLE",
        payload: {
          attacker,
          defender: targetMonster,
          cellId: targetCell.id, // нужно, чтобы потом удалить монстра при его смерти
        },
      });
      return;
    }

    // 6) Если никого не нашли
    console.log("Никого нет на этой же клетке, атака невозможна.");
  }, [state, dispatch]);

  return {
    monstersAttackPlayers,
    attackPlayerOrMonsterSameCell,
  };
}

// Если нужна вспомогательная функция для поиска монстра, оставьте (либо удалите, если она уже не используется):
export function findCellId(state: GameState, entity: Entity): number {
  const cell = state.grid.find((c) => c.monster && c.monster.id === entity.id);
  return cell ? cell.id : -1;
}
