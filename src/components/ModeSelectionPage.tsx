//=====================================
// src/components/ModeSelectionPage.tsx
//=====================================

"use client";

import React, { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "../contexts/AuthContext";
import { useDispatch } from "react-redux";
import { resetState } from "../store/slices/gameSlice";

import styles from "../styles/ModeSelectionPage.module.css";

export type GameMode = "PVE" | "1x1" | "1x2" | "2x2" | "3x3" | "5x5";
type PlayerInfo = { playerId: number; level: number };

// URL Game-сервиса
const API_MATCH = "http://localhost:8002";
const API_GAME = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8001";

const REQUIRED_SIZE: Record<GameMode, number> = {
  PVE: 1,
  "1x1": 2,
  "1x2": 3,
  "2x2": 4,
  "3x3": 6,
  "5x5": 10,
};

async function fetchQueueSize(mode: GameMode): Promise<number> {
  const res = await fetch(`${API_MATCH}/matchmaking/status?mode=${mode}`);
  if (!res.ok) throw new Error("Не удалось получить размер очереди");
  const queue: any[] = await res.json();
  return queue.length;
}

async function joinQueue(
  mode: GameMode,
  player: PlayerInfo,
  token: string
): Promise<void> {
  const res = await fetch(`${API_MATCH}/matchmaking/join`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({
      player_id: player.playerId,
      mode,
      rating: player.level * 100,
    }),
  });
  if (!res.ok) {
    throw new Error((await res.text()) || "Ошибка при вступлении в очередь");
  }
}

async function pollCurrentMatch(
  playerId: number,
  token: string
): Promise<string> {
  while (true) {
    const res = await fetch(
      `${API_MATCH}/matchmaking/currentMatch?player_id=${playerId}`,
      {
        headers: { Authorization: `Bearer ${token}` },
      }
    );
    const data = await res.json();
    if (data.instance_id) {
      return data.instance_id;
    }
    await new Promise((r) => setTimeout(r, 3000));
  }
}

export async function startMatchmaking(
  mode: GameMode,
  player: PlayerInfo,
  token: string
): Promise<string> {
  await joinQueue(mode, player, token);
  return pollCurrentMatch(player.playerId, token);
}

const MODES: { value: GameMode; label: string }[] = [
  { value: "PVE", label: "PvE (Solo)" },
  { value: "1x1", label: "PvP 1x1" },
  { value: "1x2", label: "PvP 1x2" },
  { value: "2x2", label: "PvP 2x2" },
  { value: "3x3", label: "PvP 3x3" },
  { value: "5x5", label: "PvP 5x5" },
];

export default function ModeSelectionPage() {
  const [mode, setMode] = useState<GameMode>("PVE");
  const [queueSizes, setQueueSizes] = useState<Record<GameMode, number>>({
    PVE: 0,
    "1x1": 0,
    "1x2": 0,
    "2x2": 0,
    "3x3": 0,
    "5x5": 0,
  });
  const router = useRouter();
   const { user, isLoading } = useAuth();
  const [isMatching, setIsMatching] = useState(false);
    const dispatch = useDispatch();

  useEffect(() => {
     if (window.location.pathname === "/mode") {
    dispatch(resetState());
  }
  }, [dispatch]);


  useEffect(() => {
    let mounted = true;
    async function updateAll() {
      try {
        const entries = await Promise.all(
          (Object.keys(REQUIRED_SIZE) as GameMode[]).map(async (m) => {
            const size = await fetchQueueSize(m);
            return [m, size] as [GameMode, number];
          })
        );
        if (!mounted) return;
        const sizes = Object.fromEntries(entries) as Record<GameMode, number>;
        setQueueSizes(sizes);
      } catch (e) {
        console.error(e);
      }
    }
    updateAll();
    const timer = setInterval(updateAll, 5000);
    return () => {
      mounted = false;
      clearInterval(timer);
    };
  }, []);

  useEffect(() => {
    if (!user) {
      router.push("/login");
    }
  }, [user, router]);

  useEffect(() => {
  if (!user) return;
  async function checkQueueStatus() {
    // 1. Проверяем, есть ли уже матч (уже есть эта логика)
    if (!user) return;
    const matchRes = await fetch(
      `${API_MATCH}/matchmaking/currentMatch?player_id=${user.id}`,
      { headers: { Authorization: `Bearer ${user.token}` } }
    );
    const matchData = await matchRes.json();
    if (matchData.instance_id) {
      router.push(`/game?instance_id=${matchData.instance_id}`);
      return;
    }

    // 2. Проверяем, стоит ли пользователь в очереди
    const queueRes = await fetch(
      `${API_MATCH}/matchmaking/inQueue?player_id=${user.id}`
    );
    const queueData = await queueRes.json();
    if (queueData.inQueue && queueData.mode) {
      setIsMatching(true);
      setMode(queueData.mode); // чтобы подсветить режим
    } else {
      setIsMatching(false);
    }
  }

  checkQueueStatus();
}, [user]);


  const token = user?.token;

  function isTokenExpired(token: string) {
    try {
      // JWT = header.payload.signature
      const [, payload] = token.split(".");
      if (!payload) return true;
      // В браузере atob декодирует Base64
      const decoded = JSON.parse(atob(payload));
      // exp — время в секундах с эпохи Unix
      return decoded.exp * 1000 < Date.now();
    } catch (e) {
      // Если не получилось распарсить — считаем, что токен невалиден/просрочен
      return true;
    }
  }

  async function handleStart() {
    if (!token) {
      alert("Сначала выполните вход");
      router.push("/login");
      return;
    }
    // Проверка по времени
    if (isTokenExpired(token)) {
      alert("Сессия истекла, пожалуйста, войдите снова");
      localStorage.removeItem("user"); // Очистим устаревшие данные
      router.push("/login");
      return;
    }
    try {
      setIsMatching(true);
      const playerInfo: PlayerInfo = { playerId: user!.id, level: user!.level };
      // передаём токен
      const instanceId = await startMatchmaking(mode, playerInfo, token);

      router.push(`/game?instance_id=${instanceId}`);
    } catch (err) {
      console.error(err);
      setIsMatching(false);
      alert("Ошибка при подборе игроков");
    }
  }

  async function handleCancel() {
    const token = user?.token;
    const playerId = user?.id;
    if (!token || !playerId) {
      alert("Сначала выполните вход");
      router.push("/login");
      return;
    }

    try {
      setIsMatching(false);

      const res = await fetch(`${API_MATCH}/matchmaking/cancel`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          player_id: playerId,
          mode: mode,
        }),
      });

      if (!res.ok) {
        setIsMatching(true);
        const text = await res.text();
        throw new Error(text || "Не удалось выйти из очереди");
      }
    } catch (error) {
      console.error(error);
      alert(`Ошибка при выходе из очереди: ${error}`);
    }
  }

  return (
    <div style={{ padding: "2rem" }}>
      <h2>Выберите режим:</h2>
      <div className={styles.modeContainer}>
        {(Object.keys(REQUIRED_SIZE) as GameMode[]).map((m) => (
          <div
            key={m}
            className={`${styles.modeBlock} ${
              mode === m ? styles.modeBlockSelected : ""
            }`}
            onClick={() => setMode(m)}
          >
            <div>{m === "PVE" ? "PvE (Solo)" : `PvP ${m}`}</div>
            <div style={{ marginTop: "0.5rem", fontSize: "0.9rem" }}>
              {queueSizes[m]} / {REQUIRED_SIZE[m]}
            </div>
          </div>
        ))}
      </div>

      <div className={styles.buttonGroup}>
        <button
          className={styles.queueButton}
          onClick={handleStart}
          disabled={isMatching}
        >
          Вступить в очередь
        </button>
        <button
          className={styles.queueButton}
          onClick={handleCancel}
          disabled={!isMatching}
        >
          Выйти из очереди
        </button>
      </div>
    </div>
  );
}
