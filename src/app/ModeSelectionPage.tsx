"use client";

import React, { useState } from "react";
import { startMatchmaking } from "../logic/matchmaking";
import { useRouter } from "next/navigation";

export default function ModeSelectionPage() {
  const [mode, setMode] = useState<"PVE"|"1v1"|"3v3"|"5v5">("PVE");
  const router = useRouter();

  async function handleStart() {
    // Имитируем вызов матчмейкинга
    const instanceId = await startMatchmaking(mode, { playerId: 123, level: 10 }); 
    // instanceId - это результат работы матчмейкинга (идентификатор инстанса)
    router.push(`/game?instanceId=${instanceId}`);
  }

  return (
    <div>
      <h2>Выберите режим:</h2>
      <select value={mode} onChange={(e) => setMode(e.target.value as any)}>
        <option value="PVE">PvE (Solo)</option>
        <option value="1v1">PvP 1vs1</option>
        <option value="3v3">PvP 3vs3</option>
        <option value="5v5">PvP 5vs5</option>
      </select>
      <button onClick={handleStart}>Начать подбор игроков</button>
    </div>
  );
}
