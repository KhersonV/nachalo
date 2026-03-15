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

    // Reset local turn countdown whenever turn ownership changes.
    useEffect(() => {
        turnStartMsRef.current = Date.now();
        autoEndTurnInFlightRef.current = false;
    }, [state.active_user, state.turnNumber]);

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
            <div
                className={`${styles.turnStatusFloating} ${isMyTurn ? styles.turnStatusFloatingActive : styles.turnStatusFloatingWaiting}`}
            >
                {isMyTurn ? "ТВОЙ ХОД" : "ОЖИДАНИЕ ХОДА"}
            </div>
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
                        onCellClick={handleCellClick}
                        onPlayerClick={handlePlayerClick}
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
