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
import QuestArtifactAlert from "./QuestArtifactAlert";
import styles from "../styles/GameController.module.css";
import type { RootState } from "../store";
import {
    setInstanceId,
    setActiveUser,
    setQuestFoundNotification,
} from "../store/slices/gameSlice";
import { usePlayerActions } from "../hooks/usePlayerActions";
import { useGameKeyboard } from "../hooks/useGameKeyboard";

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

    useEffect(() => {
        if (typeof window === "undefined") return;

        const updateViewport = () => {
            const screenW = window.innerWidth;
            const screenH = window.innerHeight;
            const isMobile = screenW <= 900;

            if (!isMobile) {
                setMapViewport({ width: 800, height: 600, tileSize: 80 });
                return;
            }

            const horizontalPadding = 26;
            const verticalReserved = 330;
            const width = Math.max(
                290,
                Math.min(760, screenW - horizontalPadding),
            );
            const height = Math.max(
                290,
                Math.min(560, Math.floor(screenH - verticalReserved)),
            );

            const tileSize = screenW <= 420 ? 42 : screenW <= 560 ? 48 : 56;

            setMapViewport({ width, height, tileSize });
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

    const {
        myPlayer,
        isMyTurn,
        handleMoveOrAttack,
        handleCellClick,
        handlePlayerClick,
        openBarrel,
        collectResource,
        fightMonster,
    } = usePlayerActions(instanceId, user, state);

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

    return (
        <div className={styles.container}>
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
            <div className={styles.mapContainer}>
                {myPlayer ? (
                    <MapWithCamera
                        tileSize={mapViewport.tileSize}
                        viewportWidth={mapViewport.width}
                        viewportHeight={mapViewport.height}
                        myPlayer={myPlayer}
                        onCellClick={handleCellClick}
                        onPlayerClick={handlePlayerClick}
                    />
                ) : (
                    <p className={styles.mapLoading}>Загрузка карты...</p>
                )}
            </div>
            <div className={styles.controlsContainer}>
                {isMyTurn ? (
                    <>
                        <Controls
                            onMove={handleMoveOrAttack}
                            onAction={handleAction}
                        />
                        <EndTurnButton
                            playerId={myPlayer?.user_id!}
                            instanceId={instanceId}
                            onTurnEnded={handleTurnEnded}
                        />
                        <TurnIndicator />
                        <button
                            type="button"
                            className={`${styles.inventoryButton} ${showInventory ? styles.inventoryButtonActive : ""}`}
                            onClick={() => setShowInventory((v) => !v)}
                        >
                            {showInventory
                                ? "Закрыть инвентарь"
                                : "Открыть инвентарь"}
                        </button>
                    </>
                ) : (
                    <div className={styles.waitingOverlay}>
                        <p>Ожидание хода...</p>
                        <button
                            type="button"
                            className={`${styles.inventoryButton} ${showInventory ? styles.inventoryButtonActive : ""}`}
                            onClick={() => setShowInventory((v) => !v)}
                        >
                            {showInventory
                                ? "Закрыть инвентарь"
                                : "Открыть инвентарь"}
                        </button>
                    </div>
                )}
            </div>
            {showInventory && <Inventory />}
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
        </div>
    );
}
