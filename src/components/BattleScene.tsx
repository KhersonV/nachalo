// BattleScene.tsx
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
  // Позиции
  const [attackerPos, setAttackerPos] = useState<Position>({
    x: 0,
    y: Math.floor(gridSize / 2),
  });
  const [defenderPos, setDefenderPos] = useState<Position>({
    x: gridSize - 1,
    y: Math.floor(gridSize / 2),
  });

  // Здоровье
  const [attackerHealth, setAttackerHealth] = useState(attacker.health);
  const [defenderHealth, setDefenderHealth] = useState(defender.health);

  // Чья очередь: "attacker" или "defender"
  const [turn, setTurn] = useState<"attacker" | "defender">("attacker");

  const [hasBattleEnded, setHasBattleEnded] = useState(false);

  const distance = Math.abs(attackerPos.x - defenderPos.x);

  // Подсчёт урона
  function calcDamage(atk: Entity, def: Entity) {
    return Math.max(0, atk.attack - def.defense);
  }

  // Общая функция атаки: зависит от того, кто ходит
  function handleAttack() {
    // Если бой уже закончился – ничего не делаем
    if (hasBattleEnded) return;

    if (turn === "attacker") {
      if (distance <= MELEE_RANGE) {
        const dmg = calcDamage(attacker, defender);
        setDefenderHealth((prev) => prev - dmg);
        console.log(`${attacker.name} наносит ${dmg} урона ${defender.name}`);
      } else {
        console.log("Слишком далеко для атаки");
      }
    } else {
      // turn === "defender"
      if (distance <= MELEE_RANGE) {
        const dmg = calcDamage(defender, attacker);
        setAttackerHealth((prev) => prev - dmg);
        console.log(`${defender.name} наносит ${dmg} урона ${attacker.name}`);
      } else {
        console.log("Слишком далеко для атаки");
      }
    }

    endTurn();
  }

  // Общая функция движения: влево/вправо
  function handleMove(direction: "left" | "right") {
    if (hasBattleEnded) return;

    if (turn === "attacker") {
      // Двигаем атакующего
      setAttackerPos((prev) => {
        const newX =
          direction === "left"
            ? Math.max(prev.x - 1, 0)
            : Math.min(prev.x + 1, gridSize - 1);
        return { ...prev, x: newX };
      });
    } else {
      // Двигаем защитника
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

  // Завершение хода
  function endTurn() {
    setTurn((prev) => (prev === "attacker" ? "defender" : "attacker"));
  }

  // Проверка конца боя
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
                x === attackerPos.x
                  ? "red"
                  : x === defenderPos.x
                  ? "blue"
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
        Кнопки для атакующего, если он - игрок 
        и сейчас его ход
      */}
      {turn === "attacker" && isPlayer(attacker) && !hasBattleEnded && (
        <div>
          <button onClick={() => handleMove("left")}>Move Left</button>
          <button onClick={() => handleMove("right")}>Move Right</button>
          <button onClick={handleAttack}>Attack</button>
        </div>
      )}

      {/* 
        Кнопки для защитника, если он - игрок 
        и сейчас его ход
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
