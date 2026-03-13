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

// Module-level global lock: shared across ALL instances of this hook in the same
// browser tab. Prevents multiple simultaneous WebSocket opens for the same session
// (caused by React Strict Mode double-invocation or concurrent rendering).
const activeWsKeys = new Set<string>();

// Опции: передаём только instanceId
interface GameSocketOptions {
    instanceId: string;
    enabled?: boolean;
}

// Тип WS-сообщения
export interface WSMessage<Payload = any> {
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
    token?: string,
) {
    try {
        const res = await fetch(
            `${API_BASE}/game/match?instance_id=${instanceId}`,
            {
                cache: "no-store",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${token}`,
                },
            },
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
                questArtifactId: d.quest_artifact_id ?? 0,
                questArtifactName: d.quest_artifact_name ?? "",
                questArtifactImage: d.quest_artifact_image ?? "",
                questArtifactDescription: d.quest_artifact_description ?? "",
            },
        });
    } catch (err) {
        console.warn("[useGameSocket] fetchFullMatch failed:", err);
    }
}

export function useGameSocket(
    onMessage: (msg: WSMessage) => void,
    options?: GameSocketOptions,
) {
    const wsRef = useRef<WebSocket | null>(null);
    const reconnectRef = useRef<number | null>(null);
    const connectionKeyRef = useRef<string | null>(null);
    const allowReconnectRef = useRef(true);
    const onMessageRef = useRef(onMessage);

    const router = useRouter();
    const { user } = useAuth();

    useEffect(() => {
        console.log(
            "[useGameSocket] MOUNTED for",
            options?.instanceId,
            user?.token,
        );
        return () =>
            console.log("[useGameSocket] UNMOUNT for", options?.instanceId);
    }, [options?.instanceId, user?.token]);

    // Всегда актуальный обработчик
    useEffect(() => {
        onMessageRef.current = onMessage;
    }, [onMessage]);

    // Основной эффект: connect/reconnect
    useEffect(() => {
        if (!options?.instanceId || options?.enabled === false) {
            allowReconnectRef.current = false;
            if (reconnectRef.current) {
                clearTimeout(reconnectRef.current);
                reconnectRef.current = null;
            }
            wsRef.current?.close(1000);
            wsRef.current = null;
            connectionKeyRef.current = null;
            return;
        }

        const { instanceId } = options;
        const token = user?.token;
        const connectionKey = token ? `${instanceId}:${token}` : null;
        let disposed = false;
        allowReconnectRef.current = true;

        const connect = () => {
            console.log("Connecting to WS with:", {
                token,
                instanceId,
            });
            if (!token || !instanceId || disposed || !connectionKey) return;

            // Global guard: only one WebSocket open per instanceId+token at a time.
            // This handles React StrictMode double-invocation and concurrent renders.
            if (activeWsKeys.has(connectionKey)) {
                return;
            }

            const currentWs = wsRef.current;
            const sameConnectionInProgress =
                currentWs &&
                connectionKeyRef.current === connectionKey &&
                (currentWs.readyState === WebSocket.CONNECTING ||
                    currentWs.readyState === WebSocket.OPEN);
            if (sameConnectionInProgress) {
                return;
            }

            if (currentWs && currentWs.readyState !== WebSocket.CLOSED) {
                currentWs.close(1000);
            }

            activeWsKeys.add(connectionKey);
            const ws = new WebSocket(
                `${WS_URL}?token=${token}&instanceId=${instanceId}`,
            );

            wsRef.current = ws;
            connectionKeyRef.current = connectionKey;

            ws.onopen = () => {
                if (disposed) {
                    ws.close(1000);
                    return;
                }
                ws.send(JSON.stringify({ type: "JOIN_MATCH", instanceId }));
                fetchFullMatch(instanceId, onMessageRef.current, token);
            };
            ws.onmessage = (e) => {
                let msg: WSMessage;
                try {
                    msg = JSON.parse(e.data);
                } catch {
                    console.error("WS parse error:", e.data);
                    return;
                }

                if (
                    (msg.payload?.instanceId != null ||
                        msg.payload?.instance_id != null) &&
                    (msg.payload.instanceId ?? msg.payload.instance_id) !==
                        instanceId
                ) {
                    return;
                }

                switch (msg.type) {
                    case "MATCH_ENDED":
                        allowReconnectRef.current = false;
                        if (reconnectRef.current) {
                            clearTimeout(reconnectRef.current);
                            reconnectRef.current = null;
                        }
                        wsRef.current?.close(1000);
                        router.replace("/mode");
                        return;

                    case "PLAYER_DEFEATED":
                        const defeatedUserId =
                            msg.payload.userId ?? msg.payload.user_id;
                        if (defeatedUserId === user?.id) {
                            allowReconnectRef.current = false;
                            if (reconnectRef.current) {
                                clearTimeout(reconnectRef.current);
                                reconnectRef.current = null;
                            }
                            wsRef.current?.close(1000);
                            router.replace("/mode");
                        }
                        onMessageRef.current(msg);
                        return;

                    case "TURN_PASSED":
                        // Поддерживаем оба формата полей из разных обработчиков бэкенда.
                        onMessageRef.current({
                            type: "SET_ACTIVE_USER",
                            payload: {
                                instanceId:
                                    msg.payload.instanceId ??
                                    msg.payload.instance_id,
                                active_user:
                                    msg.payload.active_user ??
                                    msg.payload.userId,
                                turnNumber:
                                    msg.payload.turnNumber ??
                                    msg.payload.turn_number,
                                energy: msg.payload.energy,
                            },
                        });
                        return;

                    case "MOVE_PLAYER":
                        onMessageRef.current({
                            type: "UPDATE_PLAYER_POSITION",
                            payload: {
                                instanceId:
                                    msg.payload.instanceId ??
                                    msg.payload.instance_id,
                                userId: msg.payload.userId,
                                newPosition: msg.payload.newPosition,
                            },
                        });
                        return;

                    case "UPDATE_CELL":
                    case "UPDATE_PLAYER":
                    case "MATCH_UPDATE":
                    case "COMBAT_EXCHANGE":
                    case "SET_ACTIVE_USER":
                    case "UPDATE_INVENTORY":
                    case "RESOURCE_COLLECTED":
                    case "BARREL_RESOURCE":
                    case "BARREL_ARTIFACT":
                        onMessageRef.current(msg);
                        return;

                    default:
                        console.warn("[WS] Unknown event type:", msg.type, msg);
                        onMessageRef.current(msg);
                }
            };

            ws.onerror = (err) => {
                console.error("WebSocket error", err);
            };

            ws.onclose = (e) => {
                const wasCurrentWs = wsRef.current === ws;
                if (wasCurrentWs) {
                    wsRef.current = null;
                    connectionKeyRef.current = null;
                    // Release the global lock so other instances (or reconnect) can connect
                    activeWsKeys.delete(connectionKey);
                }
                // Don't reconnect if a newer connection has already taken over
                if (!wasCurrentWs) return;
                if (!disposed && allowReconnectRef.current && e.code !== 1000) {
                    reconnectRef.current = window.setTimeout(connect, 3000);
                }
            };
        };

        connect();

        return () => {
            disposed = true;
            allowReconnectRef.current = false;
            // при unmount или смене instanceId
            if (reconnectRef.current) {
                clearTimeout(reconnectRef.current);
                reconnectRef.current = null;
            }
            // Release global lock before closing
            if (connectionKey) activeWsKeys.delete(connectionKey);
            wsRef.current?.close(1000);
            wsRef.current = null;
            connectionKeyRef.current = null;
        };
    }, [options?.instanceId, options?.enabled, user?.token]);

    // Закрываем при логауте
    useEffect(() => {
        if (!user) {
            wsRef.current?.close(1000);
            wsRef.current = null;
        }
    }, [user]);

    return wsRef.current;
}
