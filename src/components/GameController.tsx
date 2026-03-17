//==================================
// src/components/GameController.tsx
//==================================

import { useSelector, useDispatch } from "react-redux";
import React, { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "../contexts/AuthContext";
import MapWithCamera from "./MapWithCamera";
import Controls from "./Controls";
import EndTurnButton from "./EndTurnButton";
import TurnIndicator from "./TurnIndicator";
import Inventory from "./Inventory";
import PlayerHUD from "./PlayerHUD";
import { ObjectHUD } from "./ObjectHUD";
import QuestArtifactAlert from "./QuestArtifactAlert";
import styles from "../styles/GameController.module.css";
import objectHudStyles from "../styles/ObjectHUD.module.css";
import type { RootState } from "../store";
import type { PlayerState } from "../types";
import {
    setInstanceId,
    setActiveUser,
    setQuestFoundNotification,
} from "../store/slices/gameSlice";
import { usePlayerActions } from "../hooks/usePlayerActions";
import { useGameKeyboard } from "../hooks/useGameKeyboard";

type PlacementStructureType = "scout_tower" | "turret" | "wall";

type PlacementModeState = {
    blueprintKey: string;
    structureType: PlacementStructureType;
};

const structureTypeByBlueprintKey: Record<string, PlacementStructureType> = {
    blueprint_scout_tower: "scout_tower",
    blueprint_turret: "turret",
    blueprint_wall: "wall",
};

// Default max HP for structures when backend doesn't expose maxHealth
const STRUCTURE_DEFAULT_MAX_HEALTH: Record<PlacementStructureType, number> = {
    scout_tower: 30,
    turret: 30,
    wall: 30,
};

interface GameControllerProps {
    instanceId: string;
}

const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8001";

export default function GameController({ instanceId }: GameControllerProps) {
    const dispatch = useDispatch();
    const router = useRouter();
    const state = useSelector((state: RootState) => state.game);
    const { user } = useAuth();

    useEffect(() => {
        if (state.instanceId !== instanceId) {
            dispatch(setInstanceId(instanceId));
        }
    }, [instanceId, dispatch, state.instanceId]);

    const [showInventory, setShowInventory] = useState(false);
    const [placementMode, setPlacementMode] =
        useState<PlacementModeState | null>(null);

    // HUD для объектов (монстры, постройки, другие игроки)
    const [objectHUD, setObjectHUD] = useState<{
        type: "monster" | "structure" | "player" | "object";
        name: string;
        details?: string;
        health?: number;
        maxHealth?: number;
        energy?: number;
        maxEnergy?: number;
        attack?: number;
        defense?: number;
        sightRange?: number;
        structureType?: "scout_tower" | "turret" | "wall";
        userId?: number;
        x?: number;
        y?: number;
    } | null>(null);
    const [showTurnModal, setShowTurnModal] = useState(false);
    const prevIsMyTurnRef = React.useRef(false);
    const turnModalTimerRef = React.useRef<ReturnType<
        typeof setTimeout
    > | null>(null);
    const [showQuestAlert, setShowQuestAlert] = useState(false);
    const [showQuestFoundAlert, setShowQuestFoundAlert] = useState(false);
    const [canOpenStats, setCanOpenStats] = useState(false);
    const [disconnectedDeadlines, setDisconnectedDeadlines] = useState<
        Record<number, number>
    >({});
    const [nowMs, setNowMs] = useState<number>(Date.now());
    const [mapViewport, setMapViewport] = useState({
        width: 800,
        height: 600,
        tileSize: 80,
    });
    const questAlertShownRef = React.useRef(false);
    const turnStartMsRef = React.useRef<number>(Date.now());
    const autoEndTurnInFlightRef = React.useRef(false);
    const [profileModalUserId, setProfileModalUserId] = useState<number | null>(
        null,
    );
    const [profileModalData, setProfileModalData] = useState<any | null>(null);
    const [profileModalLoading, setProfileModalLoading] = useState(false);
    const [profileModalError, setProfileModalError] = useState("");

    useEffect(() => {
        if (typeof window === "undefined") return;

        const baseViewportWidth = 800;
        const baseViewportHeight = 600;
        const baseTileSize = 80;

        const updateViewport = () => {
            const screenW = window.innerWidth;
            const screenH = window.innerHeight;
            const isMobile = screenW <= 900;

            if (!isMobile) {
                setMapViewport({
                    width: baseViewportWidth,
                    height: baseViewportHeight,
                    tileSize: baseTileSize,
                });
                return;
            }

            // Keep the same logical map window for all devices and only scale
            // the rendered result on small screens.
            const horizontalPadding = 18;
            const verticalReserved = 190;
            const availableWidth = Math.max(280, screenW - horizontalPadding);
            const availableHeight = Math.max(220, screenH - verticalReserved);

            const scale = Math.max(
                0.42,
                Math.min(
                    1,
                    Math.min(
                        availableWidth / baseViewportWidth,
                        availableHeight / baseViewportHeight,
                    ),
                ),
            );

            setMapViewport({
                width: Math.floor(baseViewportWidth * scale),
                height: Math.floor(baseViewportHeight * scale),
                tileSize: Math.max(28, Math.floor(baseTileSize * scale)),
            });
        };

        updateViewport();
        window.addEventListener("resize", updateViewport);
        window.addEventListener("orientationchange", updateViewport);

        return () => {
            window.removeEventListener("resize", updateViewport);
            window.removeEventListener("orientationchange", updateViewport);
        };
    }, []);

    useEffect(() => {
        // Новый матч: сбрасываем флаг и возможный кэш прошлой статистики.
        setCanOpenStats(false);
        if (typeof window !== "undefined") {
            sessionStorage.removeItem("lastMatchPlayerStats");
        }
    }, [instanceId]);

    useEffect(() => {
        const handleStatsReady = () => setCanOpenStats(true);
        if (typeof window !== "undefined") {
            window.addEventListener("match-stats-ready", handleStatsReady);
            if (sessionStorage.getItem("lastMatchPlayerStats")) {
                setCanOpenStats(true);
            }
        }
        return () => {
            if (typeof window !== "undefined") {
                window.removeEventListener(
                    "match-stats-ready",
                    handleStatsReady,
                );
            }
        };
    }, []);

    useEffect(() => {
        const tick = window.setInterval(() => {
            setNowMs(Date.now());
        }, 1000);
        return () => {
            window.clearInterval(tick);
        };
    }, []);

    // Load public profile for modal when requested
    useEffect(() => {
        if (!profileModalUserId) {
            setProfileModalData(null);
            setProfileModalError("");
            setProfileModalLoading(false);
            return;
        }

        let alive = true;
        setProfileModalLoading(true);
        setProfileModalError("");
        setProfileModalData(null);

        (async () => {
            try {
                const res = await fetch(
                    `${API_BASE}/game/profile/${profileModalUserId}`,
                    {
                        headers: {
                            Authorization: `Bearer ${user?.token}`,
                        },
                        cache: "no-store",
                    },
                );
                if (!res.ok) {
                    const t = await res.text();
                    throw new Error(t || "Не удалось загрузить профиль");
                }
                const data = await res.json();
                if (!alive) return;
                setProfileModalData(data.data ?? data);
            } catch (e: any) {
                if (!alive) return;
                setProfileModalError(e?.message || "Ошибка загрузки профиля");
            } finally {
                if (!alive) return;
                setProfileModalLoading(false);
            }
        })();

        return () => {
            alive = false;
        };
    }, [profileModalUserId, user?.token]);

    // Reset local turn countdown whenever turn ownership changes.
    useEffect(() => {
        turnStartMsRef.current = Date.now();
        autoEndTurnInFlightRef.current = false;
    }, [state.active_user, state.turnNumber]);

    // Persist turn start time across navigation/refresh so client-side timer
    // doesn't reset when GameController unmounts/remounts.
    useEffect(() => {
        const key = `turnStartMs:${instanceId}`;
        if (typeof window !== "undefined") {
            try {
                const v = sessionStorage.getItem(key);
                if (v) turnStartMsRef.current = Number(v);
            } catch (e) {
                // ignore
            }
        }
        return () => {
            if (typeof window !== "undefined") {
                try {
                    sessionStorage.setItem(key, String(turnStartMsRef.current));
                } catch (e) {
                    // ignore
                }
            }
        };
    }, [instanceId]);

    useEffect(() => {
        const onDisconnected = (event: Event) => {
            const custom = event as CustomEvent<{
                userId?: number;
                graceMs?: number;
            }>;
            const userId = custom.detail?.userId;
            const graceMs = custom.detail?.graceMs ?? 180000;
            if (!userId) return;
            setDisconnectedDeadlines((prev) => ({
                ...prev,
                [userId]: Date.now() + graceMs,
            }));
        };

        const onReconnected = (event: Event) => {
            const custom = event as CustomEvent<{ userId?: number }>;
            const userId = custom.detail?.userId;
            if (!userId) return;
            setDisconnectedDeadlines((prev) => {
                const next = { ...prev };
                delete next[userId];
                return next;
            });
        };

        window.addEventListener("player-disconnected", onDisconnected);
        window.addEventListener("player-reconnected", onReconnected);

        return () => {
            window.removeEventListener("player-disconnected", onDisconnected);
            window.removeEventListener("player-reconnected", onReconnected);
        };
    }, []);

    useEffect(() => {
        const onMyDefeat = () => {
            dispatch(
                setQuestFoundNotification(
                    'Ваш персонаж погиб. Нажмите "К статистике", чтобы открыть экран результатов.',
                ),
            );
        };

        window.addEventListener("my-player-defeated", onMyDefeat);
        return () => {
            window.removeEventListener("my-player-defeated", onMyDefeat);
        };
    }, [dispatch]);

    // Show the quest artifact alert once when the map loads
    useEffect(() => {
        if (
            !questAlertShownRef.current &&
            state.isMapLoaded &&
            state.questArtifactId !== 0
        ) {
            questAlertShownRef.current = true;
            setShowQuestAlert(true);
        }
    }, [state.isMapLoaded, state.questArtifactId]);

    // Show the same modal when someone finds the quest artifact
    useEffect(() => {
        if (!state.questFoundNotification) return;
        setShowQuestFoundAlert(true);
    }, [state.questFoundNotification]);

    const handleBlueprintPlacementStart = useCallback(
        (blueprintKey: string) => {
            const structureType = structureTypeByBlueprintKey[blueprintKey];
            if (!structureType) return;
            setPlacementMode({ blueprintKey, structureType });
            setShowInventory(false);
        },
        [],
    );

    const {
        myPlayer,
        isMyTurn,
        handleMoveOrAttack,
        handleCellClick,
        handlePlayerClick,
        openBarrel,
        collectResource,
        fightMonster,
        fightPlayer,
    } = usePlayerActions(instanceId, user, state, {
        blueprintKey: placementMode?.blueprintKey ?? null,
        structureType: placementMode?.structureType ?? null,
        onPlaced: () => setPlacementMode(null),
        onError: (message: string) => {
            alert(message);
        },
    });

    const handleMapPlayerClick = useCallback(
        async (targetPlayer: PlayerState) => {
            if (!myPlayer || targetPlayer.user_id === myPlayer.user_id) return;
            const attackRange = myPlayer.attackRange ?? 1;
            const dist =
                Math.abs(myPlayer.position.x - targetPlayer.position.x) +
                Math.abs(myPlayer.position.y - targetPlayer.position.y);
            if (isMyTurn && dist <= attackRange) {
                await fightPlayer(targetPlayer.user_id);
                return;
            }
            setObjectHUD({
                type: "player",
                name: targetPlayer.name,
                health: targetPlayer.health,
                maxHealth: targetPlayer.maxHealth,
                energy: targetPlayer.energy,
                maxEnergy: targetPlayer.maxEnergy,
                attack: targetPlayer.attack,
                defense: targetPlayer.defense,
                userId: targetPlayer.user_id,
            });
        },
        [myPlayer, isMyTurn, fightPlayer],
    );

    // Show centered "YOUR TURN" modal when turn transitions to the current player
    useEffect(() => {
        if (isMyTurn && !prevIsMyTurnRef.current) {
            setShowTurnModal(true);
            if (turnModalTimerRef.current)
                clearTimeout(turnModalTimerRef.current);
            turnModalTimerRef.current = setTimeout(
                () => setShowTurnModal(false),
                2000,
            );
        }
        prevIsMyTurnRef.current = isMyTurn;
        return () => {
            if (turnModalTimerRef.current)
                clearTimeout(turnModalTimerRef.current);
        };
    }, [isMyTurn]);

    // Portal exit: call /game/finishMatch with auth token
    const handlePortalExit = useCallback(async () => {
        const token = user?.token;
        if (!token) return;
        setCanOpenStats(false);
        const res = await fetch(`${API_BASE}/game/finishMatch`, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                Authorization: `Bearer ${token}`,
            },
            body: JSON.stringify({ instanceId }),
        });
        if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            if (data?.error === "quest_artifact_missing") {
                alert("Нужно найти квест-артефакт перед выходом через портал!");
            } else {
                console.error("[Portal] finishMatch error", res.status, data);
            }
            return;
        }

        const data = await res.json().catch(() => null);
        if (typeof window !== "undefined" && data?.stats) {
            sessionStorage.setItem(
                "lastMatchPlayerStats",
                JSON.stringify(data.stats),
            );
            setCanOpenStats(true);
        }
    }, [instanceId, user]);

    // Можно оптимизировать: вынести в useCallback
    const handleAction = useCallback(() => {
        if (!isMyTurn || !myPlayer) return;
        const currentCell = state.grid.find(
            (cell: any) =>
                cell.x === myPlayer.position.x &&
                cell.y === myPlayer.position.y,
        );
        if (!currentCell) return;
        if (currentCell.monster) fightMonster(currentCell.x, currentCell.y);
        else if (currentCell.resource)
            collectResource(currentCell.x, currentCell.y);
        else if (currentCell.barbel) openBarrel(currentCell.x, currentCell.y);
        else if (currentCell.isPortal) handlePortalExit();
    }, [
        isMyTurn,
        myPlayer,
        state.grid,
        fightMonster,
        collectResource,
        openBarrel,
        handlePortalExit,
    ]);

    useGameKeyboard({
        onMove: handleMoveOrAttack,
        onAction: handleAction,
        onInventory: () => setShowInventory((v) => !v),
    });

    const handleTurnEnded = useCallback(
        (data: { active_user: number; turnNumber: number; energy: number }) => {
            turnStartMsRef.current = Date.now();
            autoEndTurnInFlightRef.current = false;
            dispatch(
                setActiveUser({
                    instanceId,
                    active_user: data.active_user,
                    turnNumber: data.turnNumber,
                    energy: data.energy,
                }),
            );
        },
        [dispatch, instanceId],
    );

    const isPortalExitNotification = !!state.questFoundNotification?.includes(
        "покинул поле боя через портал",
    );
    const isDeathNotification =
        !!state.questFoundNotification?.includes("Ваш персонаж погиб");
    const questFoundConfirmLabel =
        isDeathNotification || (isPortalExitNotification && canOpenStats)
            ? "К статистике"
            : "Понятно";

    const TURN_SECS = 60;
    const turnSecsLeft = Math.max(
        0,
        TURN_SECS - Math.floor((nowMs - turnStartMsRef.current) / 1000),
    );
    const isTurnWarning = turnSecsLeft <= 30;
    const turnTimerText = `${Math.floor(turnSecsLeft / 60)}:${String(turnSecsLeft % 60).padStart(2, "0")}`;

    // Fallback: if timer reaches 00:00 on client and it's still my turn,
    // force end-turn request to keep gameplay flowing.
    useEffect(() => {
        if (!isMyTurn || !myPlayer || turnSecsLeft > 0) return;
        if (autoEndTurnInFlightRef.current) return;

        const token = user?.token;
        if (!token) return;

        autoEndTurnInFlightRef.current = true;

        const requestAutoEndTurn = async () => {
            try {
                const response = await fetch(`${API_BASE}/game/endTurn`, {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        Authorization: `Bearer ${token}`,
                    },
                    body: JSON.stringify({
                        user_id: myPlayer.user_id,
                        instance_id: instanceId,
                    }),
                });

                if (!response.ok) {
                    autoEndTurnInFlightRef.current = false;
                    return;
                }

                const data = await response.json();
                turnStartMsRef.current = Date.now();
                dispatch(
                    setActiveUser({
                        instanceId,
                        active_user: data.active_user,
                        turnNumber: data.turn_number,
                        energy: data.energy,
                    }),
                );
            } catch {
                autoEndTurnInFlightRef.current = false;
            }
        };

        requestAutoEndTurn();
    }, [dispatch, instanceId, isMyTurn, myPlayer, turnSecsLeft, user]);

    const disconnectedPlayers = Object.entries(disconnectedDeadlines)
        .map(([id, deadline]) => {
            const userId = Number(id);
            const remainingMs = Math.max(0, deadline - nowMs);
            const remainingSec = Math.ceil(remainingMs / 1000);
            const minutes = Math.floor(remainingSec / 60);
            const seconds = remainingSec % 60;
            const playerName =
                state.players.find((p) => p.user_id === userId)?.name ??
                `Игрок ${userId}`;
            return {
                userId,
                playerName,
                remainingSec,
                isCritical: remainingSec <= 30,
                timerText: `${minutes}:${String(seconds).padStart(2, "0")}`,
            };
        })
        .filter((p) => p.remainingSec > 0);

    // Close the object HUD if the referenced target no longer exists in state
    useEffect(() => {
        if (!objectHUD) return;

        // Monster disappeared or died
        if (
            objectHUD.type === "monster" &&
            typeof objectHUD.x === "number" &&
            typeof objectHUD.y === "number"
        ) {
            const cell = state.grid.find(
                (c: any) => c.x === objectHUD.x && c.y === objectHUD.y,
            );
            if (!cell || !cell.monster) {
                setObjectHUD(null);
                return;
            }
            // If monster exists but HP dropped to 0 or less, close HUD to avoid constant re-renders
            if (
                cell.monster &&
                typeof cell.monster.health === "number" &&
                cell.monster.health <= 0
            ) {
                setObjectHUD(null);
                return;
            }
        }

        // Structure removed
        if (
            objectHUD.type === "structure" &&
            typeof objectHUD.x === "number" &&
            typeof objectHUD.y === "number"
        ) {
            const cell = state.grid.find(
                (c: any) => c.x === objectHUD.x && c.y === objectHUD.y,
            );
            if (!cell || !cell.structure_type) {
                setObjectHUD(null);
                return;
            }
            // Close HUD when structure health is zero or below
            if (
                typeof cell.structure_health === "number" &&
                cell.structure_health <= 0
            ) {
                setObjectHUD(null);
                return;
            }
        }

        // Player left or disconnected and removed
        if (objectHUD.type === "player" && objectHUD.userId) {
            const pl = state.players.find(
                (p) => p.user_id === objectHUD.userId,
            );
            if (!pl) {
                setObjectHUD(null);
                return;
            }
            // Close HUD when player HP <= 0
            if (typeof pl.health === "number" && pl.health <= 0) {
                setObjectHUD(null);
                return;
            }
        }
    }, [state.grid, state.players, objectHUD]);

    return (
        <div className={styles.container}>
            {/* HUD для выбранного объекта (монстр, постройка, игрок) */}
            {objectHUD && (
                <div className={objectHudStyles.objectHudPanel}>
                    {(() => {
                        // Derive up-to-date stats from the global game state so
                        // the HUD reflects damage/changes immediately.
                        const base = { ...objectHUD } as any;
                        if (
                            objectHUD.type === "monster" &&
                            typeof (objectHUD as any).x === "number"
                        ) {
                            const cell = state.grid.find(
                                (c: any) =>
                                    c.x === (objectHUD as any).x &&
                                    c.y === (objectHUD as any).y,
                            );
                            if (cell && cell.monster) {
                                base.name = cell.monster.name ?? base.name;
                                base.health = cell.monster.health;
                                // Preserve monster's original maxHealth so the
                                // HP bar remains relative to the true maximum.
                                if (
                                    typeof cell.monster.maxHealth === "number"
                                ) {
                                    base.maxHealth = cell.monster.maxHealth;
                                } else if (typeof base.maxHealth !== "number") {
                                    // Initialize maxHealth only once from current health
                                    base.maxHealth = cell.monster.health;
                                }
                                base.attack = cell.monster.attack;
                                base.defense = cell.monster.defense;
                            }
                        } else if (
                            objectHUD.type === "structure" &&
                            typeof (objectHUD as any).x === "number"
                        ) {
                            const cell = state.grid.find(
                                (c: any) =>
                                    c.x === (objectHUD as any).x &&
                                    c.y === (objectHUD as any).y,
                            );
                            if (cell && cell.structure_type) {
                                base.name =
                                    base.name ||
                                    (cell.structure_type === "scout_tower"
                                        ? "Башня разведки"
                                        : cell.structure_type === "turret"
                                          ? "Турель"
                                          : "Стена");
                                base.health = cell.structure_health;
                                base.maxHealth =
                                    typeof cell.structure_health === "number"
                                        ? Math.max(
                                              cell.structure_health,
                                              STRUCTURE_DEFAULT_MAX_HEALTH[
                                                  cell.structure_type as PlacementStructureType
                                              ] ?? cell.structure_health,
                                          )
                                        : STRUCTURE_DEFAULT_MAX_HEALTH[
                                              cell.structure_type as PlacementStructureType
                                          ];
                                base.defense = cell.structure_defense;
                                base.attack = cell.structure_attack;
                                base.structureType = cell.structure_type;
                            }
                        } else if (
                            objectHUD.type === "player" &&
                            objectHUD.userId
                        ) {
                            const pl = state.players.find(
                                (p) => p.user_id === objectHUD.userId,
                            );
                            if (pl) {
                                base.name = pl.name ?? base.name;
                                base.health = pl.health;
                                base.maxHealth = pl.maxHealth;
                                base.energy = pl.energy;
                                base.maxEnergy = pl.maxEnergy;
                                base.attack = pl.attack;
                                base.defense = pl.defense;
                            }
                        }

                        return (
                            <ObjectHUD
                                {...base}
                                onProfileClick={
                                    base.type === "player" && base.userId
                                        ? () => {
                                              setProfileModalUserId(
                                                  base.userId ?? null,
                                              );
                                              setObjectHUD(null);
                                          }
                                        : undefined
                                }
                                onClose={() => setObjectHUD(null)}
                            />
                        );
                    })()}
                </div>
            )}
            {profileModalUserId !== null && (
                <div
                    style={{
                        position: "fixed",
                        inset: 0,
                        background: "rgba(0,0,0,0.6)",
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                        zIndex: 1200,
                    }}
                    role="dialog"
                    aria-modal="true"
                >
                    <div
                        style={{
                            width: 720,
                            maxWidth: "96%",
                            maxHeight: "90%",
                            overflow: "auto",
                            background: "#111",
                            border: "1px solid #444",
                            padding: 16,
                            borderRadius: 8,
                        }}
                    >
                        <div
                            style={{
                                display: "flex",
                                justifyContent: "space-between",
                                alignItems: "center",
                                marginBottom: 8,
                            }}
                        >
                            <h3 style={{ margin: 0 }}>
                                {profileModalData?.player?.name ??
                                    `Игрок ${profileModalUserId}`}
                            </h3>
                            <button
                                onClick={() => setProfileModalUserId(null)}
                                style={{
                                    background: "transparent",
                                    color: "#fff",
                                    border: "1px solid #444",
                                    padding: "6px 10px",
                                    borderRadius: 4,
                                }}
                            >
                                Закрыть
                            </button>
                        </div>

                        {profileModalLoading && <div>Загрузка...</div>}
                        {profileModalError && (
                            <div style={{ color: "#f88" }}>
                                {profileModalError}
                            </div>
                        )}
                        {!profileModalLoading &&
                            !profileModalError &&
                            profileModalData && (
                                <div style={{ display: "flex", gap: 16 }}>
                                    <div style={{ width: 160 }}>
                                        <img
                                            src={
                                                profileModalData.player.image ||
                                                "/ui-icons/avatar-default.png"
                                            }
                                            alt="avatar"
                                            style={{
                                                width: 160,
                                                height: 160,
                                                objectFit: "cover",
                                                borderRadius: 8,
                                            }}
                                        />
                                    </div>
                                    <div style={{ flex: 1 }}>
                                        <div>
                                            Уровень:{" "}
                                            {profileModalData.player.level}
                                        </div>
                                        <div>
                                            Класс:{" "}
                                            {
                                                profileModalData.player
                                                    .characterType
                                            }
                                        </div>
                                        <div>
                                            Опыт:{" "}
                                            {profileModalData.player.experience}{" "}
                                            /{" "}
                                            {
                                                profileModalData.player
                                                    .maxExperience
                                            }
                                        </div>
                                        <hr
                                            style={{
                                                borderColor: "#333",
                                                margin: "8px 0",
                                            }}
                                        />
                                        <div>
                                            Атака:{" "}
                                            {profileModalData.player.attack}
                                        </div>
                                        <div>
                                            Защита:{" "}
                                            {profileModalData.player.defense}
                                        </div>
                                        <div>
                                            Подвижность:{" "}
                                            {profileModalData.player.mobility}
                                        </div>
                                        <div>
                                            Зрение:{" "}
                                            {profileModalData.player.sightRange}
                                        </div>
                                    </div>
                                </div>
                            )}
                    </div>
                </div>
            )}
            {disconnectedPlayers.length > 0 && (
                <div className={styles.disconnectBanner}>
                    <div className={styles.disconnectTitle}>
                        Отключение игрока: дается 3:00 на переподключение
                    </div>
                    <div className={styles.disconnectHint}>
                        Если таймер истечет, игрок погибнет от удара молнии.
                    </div>
                    {disconnectedPlayers.map((p) => (
                        <div
                            key={p.userId}
                            className={`${styles.disconnectRow} ${p.isCritical ? styles.disconnectRowCritical : ""}`}
                        >
                            <span>{p.playerName}</span>
                            <span
                                className={`${styles.disconnectTimer} ${p.isCritical ? styles.disconnectTimerCritical : ""}`}
                            >
                                {p.timerText}
                            </span>
                        </div>
                    ))}
                </div>
            )}
            {myPlayer && (
                <PlayerHUD
                    health={myPlayer.health}
                    maxHealth={myPlayer.maxHealth}
                    energy={myPlayer.energy}
                    maxEnergy={myPlayer.maxEnergy}
                    isRanged={myPlayer.isRanged}
                    attackRange={myPlayer.attackRange}
                />
            )}
            <div
                className={`${styles.turnStatusFloating} ${isMyTurn ? styles.turnStatusFloatingActive : styles.turnStatusFloatingWaiting}`}
            >
                {isMyTurn ? "ТВОЙ ХОД" : "ОЖИДАНИЕ ХОДА"}
            </div>
            {placementMode && (
                <div className={styles.turnStatusFloating}>
                    Режим строительства: выберите соседнюю клетку для
                    {placementMode.structureType === "scout_tower"
                        ? " башни"
                        : placementMode.structureType === "turret"
                          ? " турели"
                          : " стены"}
                </div>
            )}
            <button
                type="button"
                className={`${styles.inventoryFab} ${showInventory ? styles.inventoryFabActive : ""}`}
                onClick={() => setShowInventory((v) => !v)}
                aria-label={
                    showInventory ? "Закрыть инвентарь" : "Открыть инвентарь"
                }
                title={
                    showInventory ? "Закрыть инвентарь" : "Открыть инвентарь"
                }
            >
                <img
                    src="/ui-icons/backpack.png"
                    alt="Инвентарь"
                    className={styles.inventoryFabIcon}
                    draggable={false}
                />
            </button>
            <div className={styles.mapContainer}>
                {myPlayer ? (
                    <MapWithCamera
                        tileSize={mapViewport.tileSize}
                        viewportWidth={mapViewport.width}
                        viewportHeight={mapViewport.height}
                        myPlayer={myPlayer}
                        onCellClick={async (cell) => {
                            if (!myPlayer) return;
                            const distance =
                                Math.abs(myPlayer.position.x - cell.x) +
                                Math.abs(myPlayer.position.y - cell.y);
                            const attackRange = myPlayer.attackRange ?? 1;
                            // Монстр
                            if (cell.monster) {
                                if (isMyTurn && distance <= attackRange) {
                                    await fightMonster(cell.x, cell.y);
                                    return;
                                }
                                // Иначе показываем HUD
                                setObjectHUD({
                                    type: "monster",
                                    x: cell.x,
                                    y: cell.y,
                                    name: cell.monster.name,
                                    health: cell.monster.health,
                                    maxHealth:
                                        cell.monster.maxHealth ??
                                        cell.monster.health,
                                    attack: cell.monster.attack,
                                    defense: cell.monster.defense,
                                });
                                return;
                            }
                            // Постройка
                            if (cell.structure_type) {
                                // Бэкенд позволяет атаковать только соседние постройки (manhattan == 1)
                                // и только чужие, не строящиеся
                                const isOwnStructure =
                                    cell.structure_owner_user_id ===
                                    myPlayer.user_id;
                                if (
                                    isMyTurn &&
                                    distance === 1 &&
                                    !isOwnStructure &&
                                    !cell.is_under_construction
                                ) {
                                    await handleCellClick(cell);
                                    return;
                                }
                                setObjectHUD({
                                    type: "structure",
                                    x: cell.x,
                                    y: cell.y,
                                    name:
                                        cell.structure_type === "scout_tower"
                                            ? "Башня разведки"
                                            : cell.structure_type === "turret"
                                              ? "Турель"
                                              : "Стена",
                                    health: cell.structure_health,
                                    maxHealth:
                                        STRUCTURE_DEFAULT_MAX_HEALTH[
                                            cell.structure_type as PlacementStructureType
                                        ] ?? cell.structure_health,
                                    defense: cell.structure_defense,
                                    attack: cell.structure_attack,
                                    structureType: cell.structure_type as any,
                                    sightRange:
                                        cell.structure_type === "scout_tower"
                                            ? 1
                                            : undefined,
                                });
                                return;
                            }
                            // Ресурс
                            if (cell.resource) {
                                if (isMyTurn && distance === 1) {
                                    await handleCellClick(cell);
                                    return;
                                }
                                setObjectHUD({
                                    type: "object",
                                    name: `Ресурс: ${cell.resource.type}`,
                                    details:
                                        cell.resource.description ||
                                        "Полезный ресурс",
                                });
                                return;
                            }
                            // Бочка
                            if (cell.barbel) {
                                if (isMyTurn && distance === 1) {
                                    await handleCellClick(cell);
                                    return;
                                }
                                setObjectHUD({
                                    type: "object",
                                    name: "Бочка",
                                    details: "Можно открыть и получить награду",
                                });
                                return;
                            }
                            // Портал
                            if (cell.isPortal) {
                                setObjectHUD({
                                    type: "object",
                                    name: "Портал",
                                    details:
                                        "Точка выхода из матча после выполнения условий",
                                });
                                return;
                            }
                            // Пустая клетка — если можно дойти, двигаемся
                            if (isMyTurn && distance === 1) {
                                await handleCellClick(cell);
                                return;
                            }
                            // В остальных случаях HUD не нужен
                        }}
                        onPlayerClick={handleMapPlayerClick}
                    />
                ) : (
                    <p className={styles.mapLoading}>Загрузка карты...</p>
                )}
            </div>
            <div
                className={`${styles.controlsContainer} ${!isMyTurn ? styles.controlsContainerHiddenMobile : ""}`}
            >
                {isMyTurn ? (
                    <>
                        <div className={styles.turnPrompt}>
                            <span className={styles.turnPromptBadge}>
                                ТВОЙ ХОД
                            </span>
                            <span className={styles.turnPromptText}>
                                Выбери действие и заверши ход
                            </span>
                        </div>
                        <Controls
                            onMove={handleMoveOrAttack}
                            onAction={handleAction}
                        />
                        <TurnIndicator />
                        <div
                            className={`${styles.turnTimer} ${styles.turnTimerInline} ${isTurnWarning ? styles.turnTimerWarn : ""}`}
                        >
                            {turnTimerText}
                        </div>
                        <div className={styles.endTurnInlineDesktop}>
                            <EndTurnButton
                                playerId={myPlayer?.user_id!}
                                instanceId={instanceId}
                                onTurnEnded={handleTurnEnded}
                            />
                        </div>
                        <button
                            type="button"
                            className={`${styles.inventoryDockButton} ${showInventory ? styles.inventoryFabActive : ""}`}
                            onClick={() => setShowInventory((v) => !v)}
                            aria-label={
                                showInventory
                                    ? "Закрыть инвентарь"
                                    : "Открыть инвентарь"
                            }
                            title={
                                showInventory
                                    ? "Закрыть инвентарь"
                                    : "Открыть инвентарь"
                            }
                        >
                            <img
                                src="/ui-icons/backpack.png"
                                alt="Инвентарь"
                                className={styles.inventoryFabIcon}
                                draggable={false}
                            />
                        </button>
                    </>
                ) : (
                    <div className={styles.waitingOverlay}>
                        <div
                            className={`${styles.turnPrompt} ${styles.turnPromptWaiting}`}
                        >
                            <span
                                className={`${styles.turnPromptBadge} ${styles.turnPromptBadgeWaiting}`}
                            >
                                ОЖИДАНИЕ ХОДА
                            </span>
                            <span
                                className={`${styles.turnPromptText} ${styles.turnPromptTextWaiting}`}
                            >
                                Сейчас ход другого игрока
                            </span>
                        </div>
                        <div
                            className={`${styles.turnTimer} ${styles.turnTimerInline} ${isTurnWarning ? styles.turnTimerWarn : ""}`}
                        >
                            {turnTimerText}
                        </div>
                        <button
                            type="button"
                            className={`${styles.inventoryDockButton} ${showInventory ? styles.inventoryFabActive : ""}`}
                            onClick={() => setShowInventory((v) => !v)}
                            aria-label={
                                showInventory
                                    ? "Закрыть инвентарь"
                                    : "Открыть инвентарь"
                            }
                            title={
                                showInventory
                                    ? "Закрыть инвентарь"
                                    : "Открыть инвентарь"
                            }
                        >
                            <img
                                src="/ui-icons/backpack.png"
                                alt="Инвентарь"
                                className={styles.inventoryFabIcon}
                                draggable={false}
                            />
                        </button>
                    </div>
                )}
            </div>
            {isMyTurn && (
                <div className={styles.turnActionDock}>
                    <div
                        className={`${styles.turnTimer} ${styles.turnTimerDock} ${isTurnWarning ? styles.turnTimerWarn : ""}`}
                    >
                        {turnTimerText}
                    </div>
                    <EndTurnButton
                        playerId={myPlayer?.user_id!}
                        instanceId={instanceId}
                        onTurnEnded={handleTurnEnded}
                    />
                </div>
            )}
            {showInventory && (
                <Inventory
                    onBlueprintPlacementStart={handleBlueprintPlacementStart}
                    onRequestClose={() => setShowInventory(false)}
                />
            )}
            {showQuestAlert && (
                <QuestArtifactAlert
                    artifactId={state.questArtifactId}
                    name={state.questArtifactName}
                    image={state.questArtifactImage}
                    description={state.questArtifactDescription}
                    onClose={() => setShowQuestAlert(false)}
                />
            )}
            {showQuestFoundAlert && state.questFoundNotification && (
                <QuestArtifactAlert
                    artifactId={state.questArtifactId}
                    name={state.questArtifactName}
                    image={state.questArtifactImage}
                    description={state.questArtifactDescription}
                    badgeText="Событие"
                    hintText={state.questFoundNotification}
                    confirmLabel={questFoundConfirmLabel}
                    onClose={() => {
                        if (
                            isDeathNotification ||
                            (isPortalExitNotification && canOpenStats)
                        ) {
                            router.replace("/game/stats");
                        }
                        setShowQuestFoundAlert(false);
                        dispatch(setQuestFoundNotification(null));
                    }}
                />
            )}
            {showTurnModal && (
                <div
                    className={styles.turnModalOverlay}
                    onClick={() => setShowTurnModal(false)}
                >
                    <div className={styles.turnModalCard}>
                        <span className={styles.turnModalIcon}>⚔️</span>
                        <span className={styles.turnModalTitle}>ТВОЙ ХОД</span>
                        <span className={styles.turnModalSub}>
                            Выбери действие
                        </span>
                    </div>
                </div>
            )}
        </div>
    );
}
