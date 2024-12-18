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

const BATTLE_GRID_DEFAULT = 5;

function isPlayer(entity: Entity): entity is PlayerState {
  return "experience" in entity; // Признак, что это игрок
}

export default function BattleScene({ attacker, defender, onBattleEnd, gridSize = BATTLE_GRID_DEFAULT }: BattleSceneProps) {
  const getHealth = (entity: Entity) => {
    if ("health" in entity) {
      return { current: entity.health, max: entity.maxHealth };
    }
    throw new Error("Entity does not have health properties.");
  };
  console.log("BattleScene initialized with:", { attacker, defender });
  const [attackerPos, setAttackerPos] = useState<Position>({ x: 0, y: Math.floor(gridSize / 2) });
  const [defenderPos] = useState<Position>({ x: gridSize - 1, y: Math.floor(gridSize / 2) });
  const [attackerHealth, setAttackerHealth] = useState(getHealth(attacker).current);
  const [defenderHealth, setDefenderHealth] = useState(getHealth(defender).current);
  const [turn, setTurn] = useState<"attacker" | "defender">("attacker");
  const [isDefending, setIsDefending] = useState(false);

  const handleMove = (direction: "left" | "right") => {
    if (turn !== "attacker") return;

    setAttackerPos((prev) => {
      const newX = direction === "left" ? Math.max(prev.x - 1, 0) : Math.min(prev.x + 1, gridSize - 1);
      return { ...prev, x: newX };
    });
    endTurn();
  };

  const handleAttack = () => {
    if (turn !== "attacker") return;

    const distance = Math.abs(attackerPos.x - defenderPos.x);
    if (distance <= 1) {
      const damage = Math.max(0, attacker.attack - defender.defense);
      setDefenderHealth((prev) => prev - damage);
    }
    endTurn();
  };

  const handleDefend = () => {
    if (turn !== "attacker") return;

    setIsDefending(true);
    endTurn();
  };

  const defenderTurn = () => {
    const distance = Math.abs(attackerPos.x - defenderPos.x);
    if (distance <= 1) {
      const damage = Math.max(0, defender.attack - attacker.defense);
      setAttackerHealth((prev) => prev - (isDefending ? Math.floor(damage / 2) : damage));
    }
    setTurn("attacker");
  };

  const endTurn = () => {
    setTurn("defender");
    setIsDefending(false);
  };

  useEffect(() => {
    if (turn === "defender" && defenderHealth > 0) {
      defenderTurn();
    }
  }, [turn, defenderHealth]);

  useEffect(() => {
    if (attackerHealth <= 0) {
      onBattleEnd("defender-win", attacker);
    } else if (defenderHealth <= 0) {
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
  }, [attackerHealth, defenderHealth, onBattleEnd, attacker]);

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
        </div>
      )}
    </div>
  );
}
