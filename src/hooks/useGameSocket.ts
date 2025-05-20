//=============================
// src/hooks/useGameSocket.ts
//=============================
// src/hooks/useGameSocket.ts

import { useEffect, useRef, useCallback } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "../contexts/AuthContext";

interface GameSocketOptions {
  instanceId?: string;
}

export function useGameSocket(
  onMessage: (data: any) => void,
  options?: GameSocketOptions
) {
  const ws = useRef<WebSocket | null>(null);
  const reconnectTimeout = useRef<number | null>(null);
  const router = useRouter();
  const { user } = useAuth();
  const url = "ws://localhost:8001/ws";

  const connect = useCallback(() => {
    // Если уже открытое соединение — не открываем новое
    if (ws.current?.readyState === WebSocket.OPEN) return;

    ws.current = new WebSocket(url);

    ws.current.onopen = () => {
      // При подключении сразу посылаем JOIN_MATCH
      if (options?.instanceId) {
        ws.current?.send(
          JSON.stringify({
            type: "JOIN_MATCH",
            instanceId: options.instanceId,
          })
        );
      }
    };

    ws.current.onmessage = (event) => {
      let data: any;
      try {
        data = JSON.parse(event.data);
      } catch {
        console.error("Ошибка парсинга сообщения:", event.data);
        return;
      }

      // Игнорируем сообщения не из текущего матча
      if (
          options?.instanceId &&
          data.payload?.instanceId != null &&
          data.payload.instanceId !== options.instanceId
        ) {
        return;
      }

      // Если собственный игрок погиб — редирект на выбор режима
      if (
        data.type === "PLAYER_DEFEATED" &&
        user?.id != null &&
        data.payload?.userId === user.id
      ) {
        router.push("/mode");
        return;
      }

      onMessage(data);
    };

    ws.current.onerror = (err) => {
      console.error("WebSocket ошибка:", err);
    };

    ws.current.onclose = (e) => {
      // Попытаться переподключиться, если закрытие неавторизованное
      if (e.code !== 1000) {
        reconnectTimeout.current = window.setTimeout(connect, 3000);
      }
    };
  },
  // Важно: зависимости, включая user?.id и router
  [onMessage, options?.instanceId, router, user?.id]
  );

  useEffect(() => {
    connect();
    return () => {
      if (reconnectTimeout.current) {
        clearTimeout(reconnectTimeout.current);
      }
      ws.current?.close(1000);
    };
  }, [connect]);

  // Хуки всегда вызываются в одном и том же порядке
}
