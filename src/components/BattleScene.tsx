import "../styles/battleScene.css";
import React, { useState, useEffect } from "react";
import { Entity, PlayerState } from "../logic/types";

type Position = { x: number; y: number };

type BattleSceneProps = {
  attacker: Entity;
  defender: Entity;
  cellId: number;
  onBattleEnd: (
    result: "attacker-win" | "defender-win",
    updatedAttacker: Entity,
    cellId: number
  ) => void;
  gridSize?: number;
};

const BATTLE_GRID_DEFAULT = 7;
const MELEE_RANGE = 1; // Радиус ближней атаки для примера

function isPlayer(entity: Entity): entity is PlayerState {
  return "level" in entity;
}

export default function BattleScene({
  attacker,
  defender,
  cellId,
  onBattleEnd,
  gridSize = BATTLE_GRID_DEFAULT,
}: BattleSceneProps) {
  // Позиции на мини-поле
  const [attackerPos, setAttackerPos] = useState<Position>({
    x: 0,
    y: Math.floor(gridSize / 2),
  });
  const [defenderPos, setDefenderPos] = useState<Position>({
    x: gridSize - 1,
    y: Math.floor(gridSize / 2),
  });

  // Текущее здоровье
  const [attackerHealth, setAttackerHealth] = useState(attacker.health);
  const [defenderHealth, setDefenderHealth] = useState(defender.health);

  // Чья очередь: "attacker" или "defender"
  const [turn, setTurn] = useState<"attacker" | "defender">("attacker");

  // Флаг конца боя
  const [hasBattleEnded, setHasBattleEnded] = useState(false);

  // Упрощённая дистанция (по оси X)
  const distance = Math.abs(attackerPos.x - defenderPos.x);

  // --------------------------------------------------------------------------------
  // Вспомогательные функции
  // --------------------------------------------------------------------------------
  function calcDamage(atk: Entity, def: Entity) {
    return Math.max(0, atk.attack - def.defense);
  }

  // Атакующий наносит урон защитнику
  function doAttack(atk: Entity, def: Entity, setDefHP: React.Dispatch<React.SetStateAction<number>>) {
    const dmg = calcDamage(atk, def);
    console.log(`${atk.name} наносит ${dmg} урона ${def.name}`);
    setDefHP((prev) => prev - dmg);
  }

  // Двигаем сущность (атакующего или защитника) на 1 клетку ближе по оси X
  function moveCloser(pos: Position, targetPos: Position): Position {
    if (pos.x < targetPos.x) {
      return { ...pos, x: pos.x + 1 };
    } else if (pos.x > targetPos.x) {
      return { ...pos, x: pos.x - 1 };
    }
    // Если x уже совпадает, можно двигаться по y, но здесь упрощённый вариант
    return pos;
  }

  // --------------------------------------------------------------------------------
  // Логика движения/атаки для монстра
  // --------------------------------------------------------------------------------
  function handleMonsterTurn(isAttackerMonster: boolean) {
    if (isAttackerMonster) {
      // attacker - монстр
      if (distance <= MELEE_RANGE) {
        // Атаковать защитника
        doAttack(attacker, defender, setDefenderHealth);
      } else {
        // Сблизиться
        setAttackerPos((prev) => moveCloser(prev, defenderPos));
      }
    } else {
      // defender - монстр
      if (distance <= MELEE_RANGE) {
        // Атаковать атакующего
        doAttack(defender, attacker, setAttackerHealth);
      } else {
        // Сблизиться
        setDefenderPos((prev) => moveCloser(prev, attackerPos));
      }
    }
    // После действия – передаём ход
    endTurn();
  }

  // --------------------------------------------------------------------------------
  // Общая функция атаки, если ход у игрока (по кнопке)
  // --------------------------------------------------------------------------------
  function handleAttack() {
    if (hasBattleEnded) return;

    if (turn === "attacker") {
      // attacker ходит
      if (distance <= MELEE_RANGE) {
        doAttack(attacker, defender, setDefenderHealth);
      } else {
        console.log(`${attacker.name} слишком далеко для атаки`);
      }
    } else {
      // defender ходит
      if (distance <= MELEE_RANGE) {
        doAttack(defender, attacker, setAttackerHealth);
      } else {
        console.log(`${defender.name} слишком далеко для атаки`);
      }
    }
    endTurn();
  }

  // --------------------------------------------------------------------------------
  // Общая функция движения по кнопке (только если игрок)
  // --------------------------------------------------------------------------------
  function handleMove(direction: "left" | "right") {
    if (hasBattleEnded) return;

    if (turn === "attacker") {
      setAttackerPos((prev) => {
        const newX =
          direction === "left"
            ? Math.max(prev.x - 1, 0)
            : Math.min(prev.x + 1, gridSize - 1);
        return { ...prev, x: newX };
      });
    } else {
      setDefenderPos((prev) => {
        const newX =
          direction === "left"
            ? Math.max(prev.x - 1, 0)
            : Math.min(prev.x + 1, gridSize - 1);
        return { ...prev, x: newX };
      });
    }
    endTurn();
  }

  // --------------------------------------------------------------------------------
  // Завершаем ход
  // --------------------------------------------------------------------------------
  function endTurn() {
    setTurn((prev) => (prev === "attacker" ? "defender" : "attacker"));
  }

  // --------------------------------------------------------------------------------
  // Проверка конца боя (HP <= 0)
  // --------------------------------------------------------------------------------
  useEffect(() => {
    if (!hasBattleEnded && (attackerHealth <= 0 || defenderHealth <= 0)) {
      setHasBattleEnded(true);

      if (attackerHealth <= 0) {
        console.log(`${attacker.name} погиб! Победитель ${defender.name}`);
        onBattleEnd("defender-win", { ...defender, health: defenderHealth }, cellId);
      } else {
        console.log(`${defender.name} погиб! Победитель ${attacker.name}`);
        onBattleEnd("attacker-win", { ...attacker, health: attackerHealth }, cellId);
      }
    }
  }, [attackerHealth, defenderHealth, hasBattleEnded, onBattleEnd, attacker, defender, cellId]);

  // --------------------------------------------------------------------------------
  // Автоматический ход монстра через useEffect
  // --------------------------------------------------------------------------------
  useEffect(() => {
    if (hasBattleEnded) return;

    if (turn === "attacker" && !isPlayer(attacker)) {
      // attacker - монстр, делаем автоход
      handleMonsterTurn(true);
    } else if (turn === "defender" && !isPlayer(defender)) {
      // defender - монстр
      handleMonsterTurn(false);
    }
  }, [
    turn,
    attacker,
    defender,
    distance,
    hasBattleEnded,
    attackerPos,
    defenderPos,
    // handleMonsterTurn - если вынести её во внешний scope, 
    // можно добавлять в зависимости, но тогда нужно обернуть в useCallback
  ]);

  // --------------------------------------------------------------------------------
  // Рендер
  // --------------------------------------------------------------------------------
  return (
    <div className="battle-scene">
      <h3>Бой: {attacker.name} против {defender.name}</h3>
      <div style={{ display: "flex", marginBottom: 10 }}>
        {[...Array(gridSize)].map((_, x) => (
          <div
            key={x}
            style={{
              width: 40,
              height: 40,
              border: "1px solid black",
              backgroundColor:
                x === attackerPos.x ? "red"
                : x === defenderPos.x ? "blue"
                : "white",
            }}
          />
        ))}
      </div>

      <p>
        {attacker.name} (HP: {attackerHealth})
      </p>
      <p>
        {defender.name} (HP: {defenderHealth})
      </p>
      <p>Ход: {turn === "attacker" ? attacker.name : defender.name}</p>

      {/*
        Кнопки для атакующего, если он - игрок, и сейчас его ход
      */}
      {turn === "attacker" && isPlayer(attacker) && !hasBattleEnded && (
        <div>
          <button onClick={() => handleMove("left")}>Move Left</button>
          <button onClick={() => handleMove("right")}>Move Right</button>
          <button onClick={handleAttack}>Attack</button>
        </div>
      )}

      {/*
        Кнопки для защитника, если он - игрок, и сейчас его ход
      */}
      {turn === "defender" && isPlayer(defender) && !hasBattleEnded && (
        <div>
          <button onClick={() => handleMove("left")}>Move Left</button>
          <button onClick={() => handleMove("right")}>Move Right</button>
          <button onClick={handleAttack}>Attack</button>
        </div>
      )}
    </div>
  );
}
