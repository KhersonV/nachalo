
//===========================
// src/hooks/useGameSocket.ts
//===========================

import { useEffect, useRef, useCallback } from "react";

export function useGameSocket(onMessage: (data: any) => void) {
  const ws = useRef<WebSocket | null>(null);
  const reconnectTimeout = useRef<NodeJS.Timeout | null>(null);
  const url = "ws://localhost:8001/ws";

  const connect = useCallback(() => {
    ws.current = new WebSocket(url);

    ws.current.onopen = () => {
      console.log("WebSocket подключён.");
      // При успешном подключении можно, например, отправить запрос на синхронизацию состояния
    };

    ws.current.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        console.log("Получено сообщение по WebSocket:", data);
        onMessage(data);
      } catch (e) {
        console.error("Ошибка парсинга сообщения:", e);
      }
    };

    ws.current.onerror = (error) => {
      console.error("WebSocket ошибка:", error);
    };

    ws.current.onclose = (event) => {
      console.log("WebSocket соединение закрыто:", event.code, event.reason);
      // Если соединение закрыто не по инициативе клиента, попробуйте переподключиться
      if (event.code !== 1000) {
        // Простая логика переподключения с задержкой (например, 3 секунды)
        reconnectTimeout.current = setTimeout(() => {
          console.log("Попытка переподключения...");
          connect();
        }, 3000);
      }
    };
  }, [onMessage]);

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
