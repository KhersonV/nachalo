//===============================
// src/hooks/usePlayerActions.ts
//===============================

import { API_BASE } from "@/utils/serviceUrls";
import { useDispatch } from "react-redux";
import {
    addActionLogEntry,
    updatePlayer,
    updateCell,
} from "../store/slices/gameSlice";
import type { PlayerState, Cell } from "../types";
import { useCallback, useMemo } from "react";
import type { RootState } from "../store";
import type { User } from "../contexts/AuthContext";
import { humanizeActionError } from "@/utils/actionLog";

const DELTAS = {
    up: { x: 0, y: -1 },
    down: { x: 0, y: 1 },
    left: { x: -1, y: 0 },
    right: { x: 1, y: 0 },
} as const;

type MoveDirection = keyof typeof DELTAS;

type PlacementStructureType = "scout_tower" | "turret" | "wall";

type PlacementModeOptions = {
    blueprintKey: string | null;
    structureType: PlacementStructureType | null;
    onPlaced: () => void;
    onError: (message: string) => void;
};

export function usePlayerActions(
    instanceId: string,
    user: User | null,
    state: RootState["game"],
    placementMode?: PlacementModeOptions,
): {
    myPlayer: PlayerState | undefined;
    isMyTurn: boolean;
    handleMoveOrAttack: (direction: MoveDirection) => Promise<void>;
    handleCellClick: (cell: Cell) => Promise<void>;
    handlePlayerClick: (targetPlayer: PlayerState) => Promise<void>;
    movePlayer: (x: number, y: number) => Promise<PlayerState | null>;
    fightMonster: (cellX: number, cellY: number) => Promise<void>;
    fightPlayer: (targetUserId: number) => Promise<void>;
    openBarrel: (cellX: number, cellY: number) => Promise<any>;
    collectResource: (cellX: number, cellY: number) => Promise<void>;
} {
    const dispatch = useDispatch();

    const logActionError = useCallback(
        (
            message: string,
            category:
                | "error"
                | "movement"
                | "attack"
                | "resource"
                | "barrel" = "error",
        ) => {
            dispatch(
                addActionLogEntry({
                    category,
                    tone: "warning",
                    message,
                    dedupeKey: `action-error:${category}:${message}:${Math.floor(Date.now() / 1400)}`,
                }),
            );
        },
        [dispatch],
    );

    const readFailureMessage = useCallback(
        async (response: Response, fallback: string) => {
            const text = await response.text().catch(() => "");
            return humanizeActionError(text, fallback);
        },
        [],
    );

    const myPlayerId = user?.id;
    const myPlayer = useMemo(
        () => state.players.find((p) => p.user_id === myPlayerId),
        [state.players, myPlayerId],
    );
    const singlePlayerFallback =
        state.players.length === 1 && state.players[0]?.user_id === myPlayerId;
    const isMyTurn =
        !!myPlayer &&
        (myPlayerId === state.active_user ||
            ((!state.active_user || state.active_user === 0) &&
                singlePlayerFallback));

    // ===== Все методы ниже как у тебя, можно просто скопировать! =====
    const movePlayer = useCallback(
        async (newPosX: number, newPosY: number) => {
            try {
                const storedUser = localStorage.getItem("user");
                const token = storedUser ? JSON.parse(storedUser).token : "";
                if (!token) {
                    logActionError("Session missing. Log in again.");
                    return null;
                }
                const url = `${API_BASE}/game/${instanceId}/player/${myPlayerId}/move`;
                const body = { new_pos_x: newPosX, new_pos_y: newPosY };
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        Authorization: `Bearer ${token}`,
                    },
                    body: JSON.stringify(body),
                });
                if (!response.ok) {
                    logActionError(
                        await readFailureMessage(response, "Move failed."),
                        "movement",
                    );
                    return null;
                }
                const updatedPlayer = await response.json();
                return updatedPlayer as PlayerState;
            } catch {
                logActionError("Move failed. Check your connection.", "movement");
                return null;
            }
        },
        [instanceId, myPlayerId, logActionError, readFailureMessage],
    );

    const fightMonster = useCallback(
        async (cellX: number, cellY: number) => {
            const targetCell = state.grid.find(
                (c) => c.x === cellX && c.y === cellY,
            );
            if (!targetCell?.monster) {
                logActionError("There is no monster on that tile.", "attack");
                return;
            }
            const token = user?.token;
            if (!token || !myPlayerId) {
                logActionError("Cannot attack right now.", "attack");
                return;
            }
            try {
                const response = await fetch(`${API_BASE}/game/attack`, {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        Authorization: `Bearer ${token}`,
                    },
                    body: JSON.stringify({
                        instance_id: instanceId,
                        attacker_type: "player",
                        attacker_id: myPlayerId,
                        target_type: "monster",
                        target_id: targetCell.monster.db_instance_id,
                    }),
                });
                if (!response.ok) {
                    logActionError(
                        await readFailureMessage(response, "Attack failed."),
                        "attack",
                    );
                }
            } catch {
                logActionError("Attack failed. Check your connection.", "attack");
            }
        },
        [user, state.grid, instanceId, myPlayerId, logActionError, readFailureMessage],
    );

    const fightPlayer = useCallback(
        async (targetUserId: number) => {
            const token = user?.token;
            if (!token || !myPlayerId || targetUserId === myPlayerId) {
                logActionError("Cannot attack that player.", "attack");
                return;
            }
            try {
                const response = await fetch(`${API_BASE}/game/attack`, {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        Authorization: `Bearer ${token}`,
                    },
                    body: JSON.stringify({
                        instance_id: instanceId,
                        attacker_type: "player",
                        attacker_id: myPlayerId,
                        target_type: "player",
                        target_id: targetUserId,
                    }),
                });
                if (!response.ok) {
                    logActionError(
                        await readFailureMessage(response, "Attack failed."),
                        "attack",
                    );
                }
            } catch {
                logActionError("Attack failed. Check your connection.", "attack");
            }
        },
        [user, instanceId, myPlayerId, logActionError, readFailureMessage],
    );

    const openBarrel = useCallback(
        async (cellX: number, cellY: number) => {
            const stored = localStorage.getItem("user");
            const token = stored ? JSON.parse(stored).token : "";
            if (!token) {
                logActionError("Session missing. Log in again.");
                return null;
            }
            let res: Response;
            try {
                res = await fetch(`${API_BASE}/game/openBarrel`, {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        Authorization: `Bearer ${token}`,
                    },
                    body: JSON.stringify({
                        instance_id: instanceId,
                        user_id: myPlayerId,
                        cell_x: cellX,
                        cell_y: cellY,
                    }),
                });
            } catch {
                logActionError(
                    "Could not open the barrel. Check your connection.",
                    "barrel",
                );
                return null;
            }
            if (!res.ok) {
                logActionError(
                    await readFailureMessage(res, "Could not open the barrel."),
                    "barrel",
                );
                return null;
            }
            const data = await res.json();
            if (!data.updatedCell || !data.updatedPlayer) {
                logActionError("Barrel response was incomplete.", "barrel");
                return null;
            }
            const mappedCell: Cell = {
                cell_id: data.updatedCell.cell_id,
                x: data.updatedCell.x,
                y: data.updatedCell.y,
                tileCode: data.updatedCell.tileCode,
                resource: data.updatedCell.resource ?? null,
                barbel: data.updatedCell.barbel ?? null,
                monster: data.updatedCell.monster ?? null,
                isPortal: data.updatedCell.isPortal,
                isPlayer: data.updatedCell.isPlayer ?? false,
                structure_type: data.updatedCell.structure_type,
                structure_owner_user_id:
                    data.updatedCell.structure_owner_user_id,
                structure_health: data.updatedCell.structure_health,
                structure_defense: data.updatedCell.structure_defense,
                structure_attack: data.updatedCell.structure_attack,
                is_under_construction:
                    data.updatedCell.is_under_construction ?? false,
                construction_turns_left:
                    data.updatedCell.construction_turns_left,
            };
            const mappedPlayer: PlayerState = data.updatedPlayer as PlayerState;
            dispatch(updateCell({ instanceId, updatedCell: mappedCell }));
            dispatch(updatePlayer({ instanceId, player: mappedPlayer }));
            return { updatedCell: mappedCell, updatedPlayer: mappedPlayer };
        },
        [instanceId, myPlayerId, dispatch, logActionError, readFailureMessage],
    );

    const collectResource = useCallback(
        async (cellX: number, cellY: number) => {
            if (!instanceId || !user) {
                logActionError("Cannot collect right now.", "resource");
                return;
            }
            const token = user.token;
            if (!token) {
                logActionError("Session missing. Log in again.");
                return;
            }
            try {
                const response = await fetch(`${API_BASE}/game/collectResource`, {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        Authorization: `Bearer ${token}`,
                    },
                    body: JSON.stringify({
                        instance_id: instanceId,
                        user_id: myPlayerId,
                        cell_x: cellX,
                        cell_y: cellY,
                    }),
                });
                if (!response.ok) {
                    logActionError(
                        await readFailureMessage(
                            response,
                            "Could not collect the resource.",
                        ),
                        "resource",
                    );
                }
            } catch {
                logActionError(
                    "Could not collect the resource. Check your connection.",
                    "resource",
                );
            }
        },
        [instanceId, user, myPlayerId, logActionError, readFailureMessage],
    );

    const placeBlueprintAtCell = useCallback(
        async (cellX: number, cellY: number) => {
            if (!placementMode?.blueprintKey || !myPlayerId || !user?.token) {
                return;
            }

            const response = await fetch(`${API_BASE}/game/blueprint/place`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${user.token}`,
                },
                body: JSON.stringify({
                    instance_id: instanceId,
                    user_id: myPlayerId,
                    cell_x: cellX,
                    cell_y: cellY,
                    blueprint_key: placementMode.blueprintKey,
                }),
            });

            if (!response.ok) {
                const errorText =
                    (await response.text()) ||
                    "Не удалось начать строительство";
                placementMode.onError(errorText);
                return;
            }

            const data = await response.json();
            if (data?.updatedCell) {
                dispatch(
                    updateCell({
                        instanceId,
                        updatedCell: {
                            ...data.updatedCell,
                            resource: data.updatedCell.resource ?? null,
                            barbel: data.updatedCell.barbel ?? null,
                            monster: data.updatedCell.monster ?? null,
                            isPortal: !!data.updatedCell.isPortal,
                            isPlayer: !!data.updatedCell.isPlayer,
                            is_under_construction:
                                !!data.updatedCell.is_under_construction,
                        } as Cell,
                    }),
                );
            }
            if (data?.updatedPlayer) {
                dispatch(
                    updatePlayer({
                        instanceId,
                        player: data.updatedPlayer as PlayerState,
                    }),
                );
            }

            placementMode.onPlaced();
        },
        [dispatch, instanceId, myPlayerId, placementMode, user?.token],
    );

    const handleMoveOrAttack = useCallback(
        async (direction: MoveDirection) => {
            if (!myPlayer) return;
            if (!isMyTurn) {
                logActionError("Wait for your turn before moving.", "movement");
                return;
            }
            const { x: dx, y: dy } = DELTAS[direction];
            const targetX = myPlayer.position.x + dx;
            const targetY = myPlayer.position.y + dy;
            const targetCell = state.grid.find(
                (c) => c.x === targetX && c.y === targetY,
            );
            if (!targetCell) {
                logActionError("You cannot move outside the map.", "movement");
                return;
            }
            if (targetCell.monster) {
                await fightMonster(targetCell.x, targetCell.y);
                return;
            }

            const moved = await movePlayer(targetX, targetY);
            if (!moved) return;

            if (targetCell.resource) {
                await collectResource(targetCell.x, targetCell.y);
                return;
            }

            if (targetCell.barbel) {
                await openBarrel(targetCell.x, targetCell.y);
            }
        },
        [
            myPlayer,
            isMyTurn,
            state.grid,
            movePlayer,
            fightMonster,
            collectResource,
            openBarrel,
            logActionError,
        ],
    );

    const getDistance = useCallback(
        (cellX: number, cellY: number) => {
            if (!myPlayer) return Number.MAX_SAFE_INTEGER;
            return (
                Math.abs(myPlayer.position.x - cellX) +
                Math.abs(myPlayer.position.y - cellY)
            );
        },
        [myPlayer],
    );

    const canAttackAtDistance = useCallback(
        (distance: number) => {
            if (!myPlayer) return false;
            if (distance === 1) return true;
            return (
                !!myPlayer.isRanged && distance <= (myPlayer.attackRange ?? 1)
            );
        },
        [myPlayer],
    );

    const handlePlayerClick = useCallback(
        async (targetPlayer: PlayerState) => {
            if (
                !myPlayer ||
                !isMyTurn ||
                targetPlayer.user_id === myPlayer.user_id
            ) {
                if (myPlayer && !isMyTurn) {
                    logActionError("Wait for your turn before attacking.", "attack");
                }
                return;
            }
            const distance = getDistance(
                targetPlayer.position.x,
                targetPlayer.position.y,
            );
            if (!canAttackAtDistance(distance)) {
                logActionError("Target is out of range.", "attack");
                return;
            }
            await fightPlayer(targetPlayer.user_id);
        },
        [
            myPlayer,
            isMyTurn,
            getDistance,
            canAttackAtDistance,
            fightPlayer,
            logActionError,
        ],
    );

    const handleCellClick = useCallback(
        async (cell: Cell) => {
            if (!myPlayer) return;
            if (!isMyTurn) {
                logActionError("Wait for your turn before acting.");
                return;
            }

            if (placementMode?.blueprintKey && placementMode.structureType) {
                const distance = getDistance(cell.x, cell.y);
                if (distance !== 1) {
                    placementMode.onError(
                        "Строить можно только в соседней клетке (вверх/вниз/влево/вправо)",
                    );
                    return;
                }
                await placeBlueprintAtCell(cell.x, cell.y);
                return;
            }

            const distance = getDistance(cell.x, cell.y);
            const targetPlayer = state.players.find(
                (player) =>
                    player.user_id !== myPlayer.user_id &&
                    player.position.x === cell.x &&
                    player.position.y === cell.y,
            );

            if (cell.monster) {
                if (!canAttackAtDistance(distance)) {
                    logActionError("Monster is out of range.", "attack");
                    return;
                }
                await fightMonster(cell.x, cell.y);
                return;
            }

            if (targetPlayer) {
                if (!canAttackAtDistance(distance)) {
                    logActionError("Target is out of range.", "attack");
                    return;
                }
                await fightPlayer(targetPlayer.user_id);
                return;
            }

            if (distance === 1) {
                const moved = await movePlayer(cell.x, cell.y);
                if (!moved) return;

                if (cell.resource) {
                    await collectResource(cell.x, cell.y);
                    return;
                }

                if (cell.barbel) {
                    await openBarrel(cell.x, cell.y);
                }
                return;
            }

            logActionError("Move one adjacent tile at a time.", "movement");
        },
        [
            myPlayer,
            isMyTurn,
            getDistance,
            state.players,
            canAttackAtDistance,
            fightMonster,
            fightPlayer,
            movePlayer,
            collectResource,
            openBarrel,
            placementMode,
            placeBlueprintAtCell,
            logActionError,
        ],
    );

    return {
        myPlayer,
        isMyTurn,
        handleMoveOrAttack,
        handleCellClick,
        handlePlayerClick,
        movePlayer,
        fightMonster,
        fightPlayer,
        openBarrel,
        collectResource,
    };
}
