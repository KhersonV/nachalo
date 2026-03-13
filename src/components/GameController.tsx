//==================================
// src/components/GameController.tsx
//==================================

import { useSelector, useDispatch } from "react-redux";
import React, { useEffect, useState, useCallback } from "react";
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

export default function GameController({ instanceId }: GameControllerProps) {
    const dispatch = useDispatch();
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
    const questAlertShownRef = React.useRef(false);

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
        openBarrel,
        collectResource,
        fightMonster,
    } = usePlayerActions(instanceId, user, state);

    // Portal exit: call /game/finishMatch with auth token
    const handlePortalExit = useCallback(async () => {
        const token = user?.token;
        if (!token) return;
        const res = await fetch("http://localhost:8001/game/finishMatch", {
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
        }
        // On success the server broadcasts MATCH_ENDED which the WS handler handles
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

    const questFoundConfirmLabel = state.questFoundNotification?.includes(
        "покинул поле боя через портал",
    )
        ? "К статистике"
        : "Понятно";

    return (
        <div className={styles.container}>
            {myPlayer && (
                <PlayerHUD
                    health={myPlayer.health}
                    maxHealth={myPlayer.maxHealth}
                    energy={myPlayer.energy}
                    maxEnergy={myPlayer.maxEnergy}
                />
            )}
            <div className={styles.mapContainer}>
                {myPlayer ? (
                    <MapWithCamera
                        tileSize={80}
                        viewportWidth={800}
                        viewportHeight={600}
                        myPlayer={myPlayer}
                    />
                ) : (
                    <p>Загрузка карты...</p>
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
                    </>
                ) : (
                    <div className={styles.waitingOverlay}>
                        <p>Ожидание хода...</p>
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
                        setShowQuestFoundAlert(false);
                        dispatch(setQuestFoundNotification(null));
                    }}
                />
            )}
        </div>
    );
}
