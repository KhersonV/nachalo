

//=====================================
// src/components/ModeSelectionPage.tsx
//=====================================

"use client";

import React, { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "../contexts/AuthContext";

export type GameMode = "PVE" | "1x1" | "3x3" | "5x5";
type PlayerInfo = { playerId: number; level: number };

const API_MATCH = "http://localhost:8002";

async function joinQueue(mode: GameMode, player: PlayerInfo): Promise<void> {
  const response = await fetch(`${API_MATCH}/matchmaking/join`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      player_id: player.playerId,
      mode: mode,
      rating: player.level * 100,
    }),
  });

  if (!response.ok) {
    throw new Error("Ошибка при вступлении в очередь");
  }
}

async function pollMatchStatus(mode: GameMode, playerId: number): Promise<string> {
  while (true) {
    const res = await fetch(`${API_MATCH}/matchmaking/match?mode=${mode}&player_id=${playerId}`);
    const data = await res.json();
    if (data.instance_id) {
      return data.instance_id;
    }
    await new Promise((resolve) => setTimeout(resolve, 3000));
  }
}

export async function startMatchmaking(mode: GameMode, player: PlayerInfo): Promise<string> {
  await joinQueue(mode, player);
  const instanceId = await pollMatchStatus(mode, player.playerId);
  return instanceId;
}

export default function ModeSelectionPage() {
  const [mode, setMode] = useState<GameMode>("PVE");
  const router = useRouter();
  const { user } = useAuth();

  // Если пользователь не авторизован, перенаправляем на страницу входа
  useEffect(() => {
    if (!user) {
      router.push("/login");
    }
  }, [user, router]);

  async function handleStart() {
    try {
      // Здесь можно использовать данные пользователя (например, user.id, user.level)
      const playerInfo: PlayerInfo = { playerId: user!.id, level: 1 }; // пример, замените на реальные данные
      const instanceId = await startMatchmaking(mode, playerInfo);
      router.push(`/game?instance_id=${instanceId}`);
    } catch (error) {
      console.error(error);
      alert("Ошибка при подборе игроков");
    }
  }

  return (
    <div style={{ padding: "2rem" }}>
      <h2>Выберите режим:</h2>
      <select value={mode} onChange={(e) => setMode(e.target.value as GameMode)}>
        <option value="PVE">PvE (Solo)</option>
        <option value="1x1">PvP 1x1</option>
        <option value="3x3">PvP 3x3</option>
        <option value="5x5">PvP 5x5</option>
      </select>
      <button onClick={handleStart}>Начать подбор игроков</button>
    </div>
  );
}
