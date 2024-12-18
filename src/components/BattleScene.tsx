// src/components/BattleScene.tsx

"use client";

import React, { useState, useEffect } from "react";
import { Entity, PlayerState } from "../logic/types";

type Position = {
  x: number;
  y: number;
};

type BattleSceneProps = {
  attacker: Entity;
  defender: Entity;
  onBattleEnd: (result: "attacker-win" | "defender-win", updatedAttacker: Entity) => void;
  gridSize?: number; // Добавляем пропс для размера поля
};

const BATTLE_GRID_DEFAULT = 7; // Установите желаемый размер по умолчанию

function isPlayer(entity: Entity): entity is PlayerState {
  return "level" in entity; // Признак, что это игрок
}

export default React.memo(function BattleScene({ attacker, defender, onBattleEnd, gridSize = BATTLE_GRID_DEFAULT }: BattleSceneProps) {
  const getHealth = (entity: Entity) => {
    if ("health" in entity) {
      return { current: entity.health, max: entity.maxHealth };
    }
    throw new Error("Entity does not have health properties.");
  };

  // Логирование для отладки
  console.log("BattleScene initialized with:", { attacker, defender });

  const [attackerPos, setAttackerPos] = useState<Position>({ x: 0, y: Math.floor(gridSize / 2) });
  const [defenderPos] = useState<Position>({ x: gridSize - 1, y: Math.floor(gridSize / 2) });
  const [attackerHealth, setAttackerHealth] = useState(getHealth(attacker).current);
  const [defenderHealth, setDefenderHealth] = useState(getHealth(defender).current);
  const [turn, setTurn] = useState<"attacker" | "defender">("attacker");
  const [isDefending, setIsDefending] = useState(false);
  const [hasBattleEnded, setHasBattleEnded] = useState(false);

  const handleMove = (direction: "left" | "right") => {
    if (turn !== "attacker") return;

    console.log(`${attacker.name} пытается переместиться ${direction}`);

    setAttackerPos((prev) => {
      const newX = direction === "left" ? Math.max(prev.x - 1, 0) : Math.min(prev.x + 1, gridSize - 1);
      console.log(`${attacker.name} переместился на позицию: x=${newX}, y=${prev.y}`);
      return { ...prev, x: newX };
    });
    endTurn();
  };

  const handleAttack = () => {
    if (turn !== "attacker") return;

    const distance = Math.abs(attackerPos.x - defenderPos.x);
    console.log(`Атака: расстояние между ${attacker.name} и ${defender.name} = ${distance}`);

    if (distance <= 1) {
      const damage = Math.max(0, attacker.attack - defender.defense);
      console.log(`${attacker.name} наносит ${damage} урона ${defender.name}`);
      setDefenderHealth((prev) => {
        const newHealth = prev - damage;
        console.log(`${defender.name} теперь имеет HP = ${newHealth}`);
        return newHealth;
      });
    } else {
      console.log(`${attacker.name} не может атаковать ${defender.name}, слишком далеко.`);
    }
    endTurn();
  };

  const handleDefend = () => {
    if (turn !== "attacker") return;

    console.log(`${attacker.name} выбирает защиту.`);
    setIsDefending(true);
    endTurn();
  };

  const handlePassTurn = () => {
    if (turn !== "attacker") return;

    console.log(`${attacker.name} передает ход.`);
    endTurn();
  };

  const defenderTurn = () => {
    const distance = Math.abs(attackerPos.x - defenderPos.x);
    console.log(`Ход защитника ${defender.name}: расстояние = ${distance}`);

    if (distance <= 1) {
      const damage = Math.max(0, defender.attack - attacker.defense);
      const actualDamage = isDefending ? Math.floor(damage / 2) : damage;
      console.log(`${defender.name} наносит ${actualDamage} урона ${attacker.name} (${isDefending ? "с защитой" : ""})`);
      setAttackerHealth((prev) => {
        const newHealth = prev - actualDamage;
        console.log(`${attacker.name} теперь имеет HP = ${newHealth}`);
        return newHealth;
      });
    } else {
      console.log(`${defender.name} не может атаковать ${attacker.name}, слишком далеко.`);
    }
    setTurn("attacker");
  };

  const endTurn = () => {
    console.log(`Ход передан от ${turn} к ${turn === "attacker" ? "defender" : "attacker"}`);
    setTurn(turn === "attacker" ? "defender" : "attacker");
    setIsDefending(false);
  };

  useEffect(() => {
    if (turn === "defender" && defenderHealth > 0) {
      // Делаем задержку для имитации хода защитника
      const timer = setTimeout(() => {
        defenderTurn();
      }, 1000); // Задержка 1 секунда
      return () => clearTimeout(timer);
    }
  }, [turn, defenderHealth]);

  useEffect(() => {

    console.log(`Текущее состояние: ${attacker.name} HP=${attackerHealth}, ${defender.name} HP=${defenderHealth}`);

    if (!hasBattleEnded && (attackerHealth <= 0 || defenderHealth <= 0)) {
      if (attackerHealth <= 0) {
        console.log(`${attacker.name} погиб! ${defender.name} побеждает.`);
        onBattleEnd("defender-win", attacker);
      } else {
        console.log(`${defender.name} погиб! ${attacker.name} побеждает.`);
        let updatedAttacker: Entity;
        if (isPlayer(attacker)) {
          updatedAttacker = {
            ...attacker,
            experience: (attacker.experience || 0) + 100,
            health: attackerHealth,
          };
        } else {
          updatedAttacker = { ...attacker, health: attackerHealth };
        }
        onBattleEnd("attacker-win", updatedAttacker);
      }
      setHasBattleEnded(true);
    }
  }, [attackerHealth, defenderHealth, onBattleEnd, attacker, hasBattleEnded]);

  return (
    <div className="battle-scene">
      <div className="battle-grid" style={{ display: "flex" }}>
        {[...Array(gridSize)].map((_, x) => (
          <div
            key={x}
            className={`tile ${x === attackerPos.x ? "attacker" : ""} ${x === defenderPos.x ? "defender" : ""}`}
            style={{
              width: "50px",
              height: "50px",
              border: "1px solid black",
              display: "flex",
              justifyContent: "center",
              alignItems: "center",
            }}
          >
            {x === attackerPos.x && (isPlayer(attacker) ? <span>🧍</span> : <span>👹</span>)}
            {x === defenderPos.x && (isPlayer(defender) ? <span>🧍</span> : <span>👹</span>)}
          </div>
        ))}
      </div>
      <div className="battle-info">
        <p>
          {attacker.name}: HP {attackerHealth} / {getHealth(attacker).max}, ATK: {attacker.attack}, DEF:{" "}
          {attacker.defense}
        </p>
        <p>
          {defender.name}: HP {defenderHealth} / {getHealth(defender).max}, ATK: {defender.attack}, DEF:{" "}
          {defender.defense}
        </p>
        <p>Ход: {turn === "attacker" ? attacker.name : defender.name}</p>
      </div>
      {turn === "attacker" && (
        <div className="battle-actions">
          <button onClick={() => handleMove("left")}>⬅️ Влево</button>
          <button onClick={() => handleMove("right")}>➡️ Вправо</button>
          <button onClick={handleAttack}>🗡️ Атаковать</button>
          <button onClick={handleDefend}>🛡️ Защититься</button>
          <button onClick={handlePassTurn}>⏭️ Передать ход</button> {/* Новая кнопка для передачи хода */}
        </div>
      )}
    </div>
  );
});
