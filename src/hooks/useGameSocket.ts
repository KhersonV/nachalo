//=============================
// src/hooks/useGameSocket.ts
//=============================

import { useEffect, useRef, useCallback } from "react";

// Опциональные настройки для подключения к WebSocket
interface GameSocketOptions {
  instanceId?: string;
}

export function useGameSocket(
  onMessage: (data: any) => void,
  options?: GameSocketOptions
) {
  const ws = useRef<WebSocket | null>(null);
  const reconnectTimeout = useRef<NodeJS.Timeout | null>(null);
  const url = "ws://localhost:8001/ws"; // При продакшене этот URL можно сделать настраиваемым

  const connect = useCallback(() => {
    ws.current = new WebSocket(url);

    ws.current.onopen = () => {
      console.log("WebSocket подключён.");
      // Если передан instanceId, можно отправить сообщение для регистрации в конкретном матче
      if (options?.instanceId) {
        const joinMsg = {
          type: "JOIN_MATCH",
          instanceId: options.instanceId,
        };
        ws.current?.send(JSON.stringify(joinMsg));
        console.log("Отправлено сообщение о присоединении к матчу:", joinMsg);
      }
    };

    ws.current.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        console.log("Получено сообщение по WebSocket:", data);
        // Если указан instanceId, фильтруем входящие сообщения по нему
        if (options?.instanceId && data.payload && data.payload.instanceId) {
          if (data.payload.instanceId !== options.instanceId) {
            console.log(
              "Сообщение не относится к текущему матчу (ожидаемый instanceId:",
              options.instanceId,
              "получен:",
              data.payload.instanceId,
              "). Игнорируем."
            );
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
      console.log("WebSocket соединение закрыто:", event.code, event.reason);
      // Если соединение закрыто не по инициативе клиента (код 1000 – нормальное закрытие),
      // пробуем переподключиться через 3 секунды
      if (event.code !== 1000) {
        reconnectTimeout.current = setTimeout(() => {
          console.log("Попытка переподключения...");
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
