//=============================
// src/hooks/useGameSocket.ts
//=============================


import { useEffect, useRef } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "../contexts/AuthContext";
import { Cell, PlayerState } from "../types/GameTypes";

// -- Вынесем базовый адрес WebSocket и HTTP API в константы/ENV --
const WS_URL = process.env.NEXT_PUBLIC_WS_URL || "ws://localhost:8001/ws";
const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8001";

// Опции: передаём только instanceId
interface GameSocketOptions {
  instanceId: string;
}

// Тип WS-сообщения
interface WSMessage<Payload = any> {
  type: string;
  payload?: Payload;
}

interface MatchResponse {
  instance_id: string;
  mode: string;
  map: Cell[];
  map_width: number;
  map_height: number;
  players: PlayerState[];
  active_user: number;
  turn_number: number;
}

// Полный матч, которым мы дёргаем HTTP на onopen/reconnect
async function fetchFullMatch(
  instanceId: string,
  onMessage: (msg: WSMessage) => void,
  token?: string
) {
  try {
    const res = await fetch(
      `${API_BASE}/game/match?instance_id=${instanceId}`,
      {
        cache: "no-store",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${token}`,
        },
      }
    );
    if (!res.ok) return;
    const d = await res.json();
    onMessage({
      type: "MATCH_UPDATE",
      payload: {
        instanceId: d.instance_id,
        mode: d.mode,
        grid: d.map,
        mapWidth: d.map_width,
        mapHeight: d.map_height,
        players: d.players.map((p: any) => ({
          ...p,
          user_id: p.user_id ?? p.player_id,
          position: p.position,
        })),
        active_player: d.active_user,
        turn_number: d.turn_number,
      },
    });
  } catch (err) {
    console.warn("[useGameSocket] fetchFullMatch failed:", err);
  }
}

export function useGameSocket(
  onMessage: (msg: WSMessage) => void,
  options?: GameSocketOptions
) {
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectRef = useRef<number | null>(null);
  const onMessageRef = useRef(onMessage);

  const router = useRouter();
  const { user } = useAuth();

  // Всегда актуальный обработчик
  useEffect(() => {
    onMessageRef.current = onMessage;
  }, [onMessage]);

  // Основной эффект: connect/reconnect
  useEffect(() => {
    if (!options?.instanceId) return;

    const { instanceId } = options;

    const connect = () => {
      const ws = new WebSocket(`${WS_URL}?token=${user?.token}&instanceId=${instanceId}`);

      wsRef.current = ws;

      ws.onopen = () => {
        ws.send(JSON.stringify({ type: "JOIN_MATCH", instanceId }));
        fetchFullMatch(instanceId, onMessageRef.current, user?.token);
      };

      ws.onmessage = (e) => {
        let msg: WSMessage;
        try {
          msg = JSON.parse(e.data);
        } catch {
          console.error("WS parse error:", e.data);
          return;
        }

        // Фильтруем чужие матчи
        if (
          msg.payload?.instanceId != null &&
          msg.payload.instanceId !== instanceId
        ) {
          return;
        }

        switch (msg.type) {
          case "MATCH_ENDED":
            // матч завершён
            router.replace("/mode");
            return;

          case "PLAYER_DEFEATED":
            // если это моя смерть — выходим
            if (msg.payload.userId === user?.id) {
              router.replace("/mode");
              return;
            }
            // чужая смерть — просто прокидываем дальше, чтобы context мог обновить, например, список игроков
            break;

          case "TURN_PASSED":
            // смена хода — диспатчим в контекст
            onMessageRef.current({
              type: "SET_ACTIVE_USER",
              payload: {
                instanceId: msg.payload.instanceId,
                active_user: msg.payload.userId,
                turnNumber: msg.payload.turnNumber,
                energy: msg.payload.energy, // если сервер шлёт energy
              },
            });
            return;

          // можно добавить другие кейсы событий WS здесь…
        }

        // по умолчанию — прокидываем все остальные события в контекст
        onMessageRef.current(msg);
      };

      ws.onerror = (err) => {
        console.error("WebSocket error", err);
      };

      ws.onclose = (e) => {
        // 1000 — нормальное закрытие
        if (e.code !== 1000) {
          reconnectRef.current = window.setTimeout(connect, 3000);
        }
      };
    };

    connect();

    return () => {
      // при unmount или смене instanceId
      if (reconnectRef.current) {
        clearTimeout(reconnectRef.current);
      }
      wsRef.current?.close(1000);
      wsRef.current = null;
    };
  }, [options?.instanceId, user?.token]);

  // Закрываем при логауте
  useEffect(() => {
    if (!user) {
      wsRef.current?.close(1000);
      wsRef.current = null;
    }
  }, [user]);

  return wsRef.current;
}
