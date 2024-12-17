"use client";

import React, { useState, useEffect } from "react";

type Entity = {
  id: number;
  name: string;
  health: number;
  maxHealth: number;
  attack: number;
  defense: number;
  experience?: number;
  type: "player" | "monster"; // Тип сущности
};

type Position = {
  x: number;
  y: number;
};

type BattleSceneProps = {
  attacker: Entity; // Атакующий
  defender: Entity; // Защищающийся
  onBattleEnd: (result: "attacker-win" | "defender-win", updatedAttacker: Entity) => void;
};

const BATTLE_GRID = 5; // Размер карты 5x5

export default function BattleScene({ attacker, defender, onBattleEnd }: BattleSceneProps) {
  const [attackerPos, setAttackerPos] = useState<Position>({ x: 0, y: 2 });
  const [defenderPos] = useState<Position>({ x: 4, y: 2 });
  const [attackerHealth, setAttackerHealth] = useState(attacker.health);
  const [defenderHealth, setDefenderHealth] = useState(defender.health);
  const [turn, setTurn] = useState<"attacker" | "defender">("attacker"); // Ход атакующего
  const [isDefending, setIsDefending] = useState(false);

  const handleMove = (direction: "left" | "right") => {
    if (turn !== "attacker") return;

    setAttackerPos((prev) => {
      const newX = direction === "left" ? Math.max(prev.x - 1, 0) : Math.min(prev.x + 1, BATTLE_GRID - 1);
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
  }, [turn]);

  useEffect(() => {
    if (attackerHealth <= 0) {
      onBattleEnd("defender-win", attacker);
    } else if (defenderHealth <= 0) {
      const updatedAttacker = {
        ...attacker,
        experience: (attacker.experience || 0) + 100, // Добавляем опыт атакующему
        health: attackerHealth,
      };
      onBattleEnd("attacker-win", updatedAttacker);
    }
  }, [attackerHealth, defenderHealth]);

  return (
    <div className="battle-scene">
      <div className="battle-grid">
        {[...Array(BATTLE_GRID)].map((_, x) => (
          <div
            key={x}
            className={`tile ${x === attackerPos.x ? "attacker" : ""} ${
              x === defenderPos.x ? "defender" : ""
            }`}
          >
            {x === attackerPos.x && (attacker.type === "player" ? <span>🧍</span> : <span>👹</span>)}
            {x === defenderPos.x && (defender.type === "player" ? <span>🧍</span> : <span>👹</span>)}
          </div>
        ))}
      </div>
      <div className="battle-info">
        <p>
          {attacker.name}: HP {attackerHealth} / {attacker.maxHealth}, ATK: {attacker.attack}, DEF:{" "}
          {attacker.defense}
        </p>
        <p>
          {defender.name}: HP {defenderHealth} / {defender.maxHealth}, ATK: {defender.attack}, DEF:{" "}
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
