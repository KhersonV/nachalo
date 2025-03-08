
//===========================
// src/hooks/useGameSocket.ts
//===========================

import { useEffect, useRef } from "react";

export function useGameSocket(onMessage: (data: any) => void) {
  const ws = useRef<WebSocket | null>(null);

  useEffect(() => {
    // Укажите правильный URL для вашего сервера
    ws.current = new WebSocket("ws://localhost:8001/ws");

    ws.current.onopen = () => {
      console.log("WebSocket подключён.");
    };

    ws.current.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        onMessage(data);
      } catch (e) {
        console.error("Ошибка парсинга сообщения:", e);
      }
    };

    ws.current.onerror = (error) => {
      console.error("WebSocket ошибка:", error);
    };

    ws.current.onclose = () => {
      console.log("WebSocket соединение закрыто.");
    };

    return () => {
      ws.current?.close();
    };
  }, [onMessage]);
}
