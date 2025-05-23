//=============================
// src/hooks/useGameSocket.ts
//=============================


import { useEffect, useRef } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "../contexts/AuthContext";

interface GameSocketOptions {
  instanceId?: string;
}

export function useGameSocket(
  onMessage: (data: any) => void,
  options?: GameSocketOptions
) {
  const wsRef = useRef<WebSocket | null>(null);
  const onMessageRef = useRef(onMessage);
  const reconnectTimeout = useRef<number | null>(null);
  const router = useRouter();
  const { user } = useAuth();
  const url = "ws://localhost:8001/ws";

  // Держим актуальный onMessage в рефе
  useEffect(() => {
    onMessageRef.current = onMessage;
  }, [onMessage]);

  useEffect(() => {
    function connect() {
      const ws = new WebSocket(url);
      wsRef.current = ws;

    // в src/hooks/useGameSocket.ts, внутри ws.onopen
ws.onopen = async () => {
  if (options?.instanceId) {
    ws.send(JSON.stringify({ type: "JOIN_MATCH", instanceId: options.instanceId }));
    // <-- вставляем запрос на полные данные матча
    try {
      const res = await fetch(`http://localhost:8001/game/match?instance_id=${options.instanceId}`, { cache: "no-store" });
      if (res.ok) {
        const data = await res.json()
        // передаём наружу специальным событием
        onMessageRef.current({ type: 'MATCH_UPDATE', payload: {
          instanceId: data.instance_id,
          mode: data.mode,
          grid: data.map,
          mapWidth: data.map_width,
          mapHeight: data.map_height,
          players: data.players.map((p:any) => ({ 
            ...p, user_id: p.user_id||p.player_id, position: p.position 
          })),
          active_player: data.active_user,
          turn_number: data.turn_number
        }})
      }
    } catch (e) {
      console.warn("Не удалось подгрузить полный матч после reconnect", e);
    }
  }
};


      ws.onmessage = (event) => {
        let data: any;
        try {
          data = JSON.parse(event.data);
        } catch {
          console.error("WS parse error:", event.data);
          return;
        }

        // Игнорим пакеты не из текущего матча
        if (
          options?.instanceId &&
          data.payload?.instanceId != null &&
          data.payload.instanceId !== options.instanceId
        ) {
          return;
        }

        if (data.type === "MATCH_ENDED") {
          router.push("/mode");
          return;
        }

        if (
          data.type === "PLAYER_DEFEATED" &&
          user?.id != null &&
          data.payload?.userId === user.id
        ) {
          router.push("/mode");
          return;
        }

        onMessageRef.current(data);
      };

      ws.onerror = (e) => {
        console.error("WebSocket error", e);
      };

      ws.onclose = (e) => {
        // если закрытие «по ошибке» — переподключаемся через 3 сек
        if (e.code !== 1000) {
          reconnectTimeout.current = window.setTimeout(connect, 3000);
        } else {
          reconnectTimeout.current = null;
        }
      };
    }

    connect();

    return () => {
      if (reconnectTimeout.current) {
        clearTimeout(reconnectTimeout.current);
      }
      wsRef.current?.close(1000);
    };
  }, [options?.instanceId, router, user?.id]);

  return wsRef.current;
}
