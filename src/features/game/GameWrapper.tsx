//===================================
// src/features/game/GameWrapper.tsx
//===================================

"use client";

import React, { useEffect, useState, useRef } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import { useDispatch, useSelector } from "react-redux";
import { RootState, AppDispatch } from "@/store";
import { resetState } from "@/store/slices/gameSlice";
import { useGameSocket } from "@/hooks/useGameSocket";
import type { ResourceType, MonsterType } from "@/types/GameTypes";
import { createWsHandlers } from "./createWsHandlers";

interface GameWrapperProps {
    instanceId: string;
    children: React.ReactNode;
}

const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8001";

export default function GameWrapper({
    instanceId,
    children,
}: GameWrapperProps) {
    console.log("[GameWrapper] РЕНДЕР! instanceId:", instanceId);
    const dispatch = useDispatch<AppDispatch>();
    const router = useRouter();
    const { user, isLoading } = useAuth();
    const state = useSelector((state: RootState) => state.game);

    const [resources, setResources] = useState<ResourceType[]>([]);
    const [monsters, setMonsters] = useState<MonsterType[]>([]);
    const prevInstanceIdRef = useRef<string>("");
    const isFetching = useRef(false);

    // ========== FETCH MATCH ==========

    async function fetchAll() {
        if (isFetching.current) return;
        isFetching.current = true;

        try {
            if (!user || !user.token) {
                console.warn("[GameWrapper] Нет user или токена — выход");
                return;
            }
            const [resourcesRes, monstersRes] = await Promise.all([
                fetch(`${API_BASE}/api/resources`),
                fetch(`${API_BASE}/api/monsters`),
            ]);
            if (!resourcesRes.ok) {
                const text = await resourcesRes.text();
                throw new Error(`resources ${resourcesRes.status}: ${text}`);
            }
            if (!monstersRes.ok) {
                const text = await monstersRes.text();
                throw new Error(`monsters ${monstersRes.status}: ${text}`);
            }
            const [resources, monsters] = await Promise.all([
                resourcesRes.json(),
                monstersRes.json(),
            ]);
            setResources(resources);
            setMonsters(monsters);

            console.log("[GameWrapper] user:", user);
            console.log("[GameWrapper] fetch match...");

            const res = await fetch(
                `${API_BASE}/game/match?instance_id=${instanceId}`,
                { headers: { Authorization: `Bearer ${user.token}` } },
            );
            if (!res.ok) {
                const text = await res.text();
                throw new Error(`match ${res.status}: ${text}`);
            }
            const data = await res.json();
            console.log("[GameWrapper] match data:", data);

            const players = data.players.map((p: any) => ({
                ...p,
                user_id: p.user_id ?? p.player_id,
                position: p.position,
            }));

            let active_user = data.active_player ?? data.active_user;
            if ((!active_user || active_user === 0) && players.length > 0) {
                active_user = players[0].user_id;
            }

            if (state.instanceId === data.instance_id && state.isMapLoaded) {
                // ничего не делаем, всё уже актуально
                return;
            }

            console.log("[DISPATCH setMatchData] SOURCE: fetchAll", {
                payload: {
                    instanceId: data.instance_id,
                    mode: data.mode,
                    grid: data.map,
                    mapWidth: data.map_width,
                    mapHeight: data.map_height,
                    players,
                    active_user,
                    turnNumber: data.turn_number,
                },
            });
            dispatch({
                type: "game/setMatchData",
                payload: {
                    instanceId: data.instance_id,
                    mode: data.mode,
                    grid: data.map,
                    mapWidth: data.map_width,
                    mapHeight: data.map_height,
                    players,
                    active_user,
                    turnNumber: data.turn_number,
                },
            });
        } catch (e) {
            console.error(
                "Ошибка загрузки данных матча, ресурсов или монстров",
                e,
            );
            router.replace("/mode");
        } finally {
            isFetching.current = false;
        }
    }
    useEffect(() => {
        if (isLoading) return;
        console.log(
            "[GameWrapper/useEffect] user:",
            user,
            "token:",
            user?.token,
            "isMapLoaded:",
            state.isMapLoaded,
            "instanceId:",
            instanceId,
        );

        if (!user) {
            console.log("[GameWrapper/useEffect] Нет user, уходим на /login");
            router.replace("/login");
            return;
        }
        if (
            prevInstanceIdRef.current &&
            prevInstanceIdRef.current !== instanceId
        ) {
            console.log(
                "[GameWrapper/useEffect] Новый instanceId, resetState()",
            );
            dispatch(resetState());
        }
        prevInstanceIdRef.current = instanceId;

        if (!state.isMapLoaded && user && user.token) {
            console.log("[GameWrapper/useEffect] Запускаем fetchAll");
            fetchAll();
        } else {
            console.log(
                "[GameWrapper/useEffect] НЕ запускаем fetchAll (isMapLoaded:",
                state.isMapLoaded,
                ")",
            );
        }
    }, [instanceId, user, isLoading]);

    // ========== SOCKET ==========

    type WsEventType =
        | "MATCH_UPDATE"
        | "MOVE_PLAYER"
        | "UPDATE_CELL"
        | "UPDATE_PLAYER"
        | "COMBAT_EXCHANGE"
        | "SET_ACTIVE_USER"
        | "PLAYER_DEFEATED"
        | "TURN_PASSED"
        | "UPDATE_INVENTORY"
        | "RESOURCE_COLLECTED"
        | "BARREL_RESOURCE"
        | "BARREL_ARTIFACT"
        | "QUEST_ARTIFACT_FOUND"
        | "PLAYER_LEFT_PORTAL"
        | "PLAYER_DISCONNECTED"
        | "PLAYER_RECONNECTED"
        | "MATCH_ENDED";

    // Типизация всех известных событий:
    type WsHandlers = {
        [K in WsEventType]: (payload: any) => void;
    } & {
        // На всякий случай для всех остальных
        [key: string]: (payload: any) => void;
    };

    const wsHandlers = createWsHandlers(dispatch, router, instanceId, user);

    const socket = useGameSocket(
        (data) => {
            const payload = data.payload ?? {};
            const payloadInstanceId = payload.instanceId ?? payload.instance_id;
            if (payloadInstanceId && payloadInstanceId !== instanceId) return;

            const normalizedPayload = {
                ...payload,
                instanceId: payloadInstanceId ?? payload.instanceId,
                userId: payload.userId ?? payload.user_id,
                active_user: payload.active_user ?? payload.active_player,
                turnNumber: payload.turnNumber ?? payload.turn_number,
                grid: payload.grid ?? payload.map,
                mapWidth: payload.mapWidth ?? payload.map_width,
                mapHeight: payload.mapHeight ?? payload.map_height,
            };

            const handler = wsHandlers[data.type as WsEventType];
            if (handler) {
                handler(normalizedPayload);
            } else {
                console.warn("[WS] Неизвестный тип события:", data.type, data);
            }
        },
        { instanceId, enabled: !!user },
    );

    useEffect(() => {
        if (!user && socket) socket.close();
    }, [user, socket]);

    return <>{children}</>;
}
