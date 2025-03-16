//=============================
// src/hooks/useGameSocket.ts
//=============================

import { useEffect, useRef, useCallback } from "react";

interface GameSocketOptions {
  instanceId?: string;
}

export function useGameSocket(
  onMessage: (data: any) => void,
  options?: GameSocketOptions
) {
  const ws = useRef<WebSocket | null>(null);
  const reconnectTimeout = useRef<number | null>(null);
  // Используем протокол ws://, убедитесь, что сервер запущен и доступен
  const url = "ws://localhost:8001/ws";

  const connect = useCallback(() => {
    // Если уже есть открытое соединение, не создаём новое
    if (ws.current && ws.current.readyState === WebSocket.OPEN) {
      console.log("WebSocket уже подключён");
      return;
    }

    console.log("Попытка установить WebSocket соединение...");
    ws.current = new WebSocket(url);

    ws.current.onopen = () => {
      console.log("WebSocket соединение установлено");
      if (options?.instanceId) {
        const joinMsg = {
          type: "JOIN_MATCH",
          instanceId: options.instanceId,
        };
        ws.current?.send(JSON.stringify(joinMsg));
        console.log("Отправлено JOIN_MATCH сообщение:", joinMsg);
      }
    };

    ws.current.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        console.log("Получено сообщение по WebSocket:", data);
        if (options?.instanceId && data.payload && data.payload.instanceId) {
          if (data.payload.instanceId !== options.instanceId) {
            console.log("Сообщение не для текущего матча, игнорируем:", data.payload.instanceId);
            return;
          }
        }
        onMessage(data);
      } catch (e) {
        console.error("Ошибка парсинга сообщения:", e);
      }
    };

    ws.current.onerror = (error) => {
      console.error("WebSocket ошибка:", error);
    };

    ws.current.onclose = (event) => {
      console.log(`WebSocket соединение закрыто (код: ${event.code}, причина: ${event.reason})`);
      // Если закрытие произошло не по инициативе клиента (код 1000), пробуем переподключиться
      if (event.code !== 1000) {
        console.log("Попытка переподключения через 3 секунды...");
        reconnectTimeout.current = window.setTimeout(() => {
          connect();
        }, 3000);
      }
    };
  }, [onMessage, options]);

  useEffect(() => {
    connect();
    return () => {
      if (reconnectTimeout.current) {
        clearTimeout(reconnectTimeout.current);
      }
      ws.current?.close();
    };
  }, [connect]);
}
