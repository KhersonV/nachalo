//=======================================
// src/features/game/createWsHandlers.ts
//=======================================

import { AppDispatch } from "@/store";
import {
    setMatchData,
    updateCell,
    updatePlayer,
    movePlayer,
    combatExchange,
    setActiveUser,
    playerDefeated,
    turnPassed,
    updateInventory,
    updatePlayerPosition,
    updatePlayerHealth,
    setQuestFoundNotification,
} from "@/store/slices/gameSlice";

// The User type can be imported from your AuthContext
type User = { id: number } | null;

export function createWsHandlers(
    dispatch: AppDispatch,
    router: any,
    instanceId: string,
    user: User,
) {
    return {
        // --- MATCH UPDATE ---
        MATCH_UPDATE: (payload: any) => {
            dispatch(setMatchData(payload));
        },

        // --- MOVE/POSITION ---
        MOVE_PLAYER: (payload: any) =>
            dispatch(
                movePlayer({
                    instanceId,
                    userId: payload.playerId ?? payload.userId,
                    newPosition: payload.newPosition,
                }),
            ),
        UPDATE_PLAYER_POSITION: (payload: any) =>
            dispatch(
                updatePlayerPosition({
                    userId: payload.userId,
                    newPosition: payload.newPosition,
                }),
            ),
        BARREL_DAMAGE: (payload: any) =>
            dispatch(
                updatePlayerHealth({
                    userId: payload.userId,
                    hp: payload.hp,
                }),
            ),
        // --- CELL/PLAYER UPDATES ---
        UPDATE_CELL: (payload: any) =>
            dispatch(
                updateCell({
                    instanceId,
                    updatedCell: payload.updatedCell,
                }),
            ),
        UPDATE_PLAYER: (payload: any) =>
            dispatch(
                updatePlayer({
                    instanceId,
                    player: payload.player,
                }),
            ),
        // --- BARREL/RESOURCE ---
        RESOURCE_COLLECTED: (payload: any) => {
            dispatch(
                updateCell({ instanceId, updatedCell: payload.updatedCell }),
            );
            dispatch(
                updatePlayer({ instanceId, player: payload.updatedPlayer }),
            );
        },
        BARREL_RESOURCE: (payload: any) => {
            dispatch(
                updateCell({ instanceId, updatedCell: payload.updatedCell }),
            );
            dispatch(
                updatePlayer({ instanceId, player: payload.updatedPlayer }),
            );
        },
        BARREL_ARTIFACT: (payload: any) => {
            dispatch(
                updateCell({ instanceId, updatedCell: payload.updatedCell }),
            );
            dispatch(
                updatePlayer({ instanceId, player: payload.updatedPlayer }),
            );
        },
        // --- BATTLE/COMBAT ---
        COMBAT_EXCHANGE: (payload: any) =>
            dispatch(
                combatExchange({
                    instanceId,
                    attacker: payload.attacker,
                    target: payload.target,
                }),
            ),
        // --- TURN/USER ---
        SET_ACTIVE_USER: (payload: any) =>
            dispatch(
                setActiveUser({
                    instanceId,
                    active_user: payload.active_user,
                    turnNumber: payload.turnNumber,
                    energy: payload.energy,
                }),
            ),
        TURN_PASSED: (payload: any) =>
            dispatch(
                turnPassed({
                    instanceId,
                    active_user: payload.active_user,
                }),
            ),
        PLAYER_DEFEATED: (payload: any) => {
            dispatch(
                playerDefeated({
                    instanceId,
                    userId: payload.userId,
                }),
            );
        },
        // --- INV ---
        UPDATE_INVENTORY: (payload: any) =>
            dispatch(
                updateInventory({
                    instanceId,
                    userId: payload.userId,
                    inventory: payload.inventory,
                }),
            ),
        // --- END ---
        MATCH_ENDED: () => {
            const hasLastMatchStats =
                typeof window !== "undefined" &&
                !!sessionStorage.getItem("lastMatchPlayerStats");
            if (hasLastMatchStats) return;
            const isPendingAfterDefeat =
                typeof window !== "undefined" &&
                sessionStorage.getItem("lastMatchStatsPending") === "1";
            if (isPendingAfterDefeat) return;
            router.replace("/mode");
        },
        QUEST_ARTIFACT_FOUND: (payload: any) => {
            dispatch(
                setQuestFoundNotification({
                    eventType: "QUEST_ARTIFACT_FOUND",
                    message: `Player ${payload.playerName} found the quest artifact at cell (${payload.x}, ${payload.y}).`,
                    instanceId: payload.instanceId,
                    playerName: payload.playerName,
                    x: payload.x,
                    y: payload.y,
                }),
            );
        },
        PLAYER_LEFT_PORTAL: (payload: any) => {
            dispatch(
                setQuestFoundNotification({
                    eventType: "PLAYER_LEFT_PORTAL",
                    message: `Player ${payload.playerName} left the battlefield through the portal at cell (${payload.x}, ${payload.y}).`,
                    instanceId: payload.instanceId,
                    playerName: payload.playerName,
                    x: payload.x,
                    y: payload.y,
                }),
            );
        },
        PLAYER_DISCONNECTED: () => {
            // Handled locally in GameController via window events.
        },
        PLAYER_RECONNECTED: () => {
            // Handled locally in GameController via window events.
        },
        // ... any other new event types!
    };
}
