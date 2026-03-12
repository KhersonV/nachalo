//===============================
// src/hooks/usePlayerActions.ts
//===============================

import { useDispatch } from "react-redux";
import { updatePlayer, updateCell } from "../store/slices/gameSlice";
import type { PlayerState, Cell } from "../types/GameTypes";
import { useCallback, useMemo } from "react";
import type { RootState } from "../store";
import type { User } from "../contexts/AuthContext";

const DELTAS = {
    up: { x: 0, y: -1 },
    down: { x: 0, y: 1 },
    left: { x: -1, y: 0 },
    right: { x: 1, y: 0 },
} as const;

type MoveDirection = keyof typeof DELTAS;

export function usePlayerActions(
    instanceId: string,
    user: User | null,
    state: RootState["game"],
): {
    myPlayer: PlayerState | undefined;
    isMyTurn: boolean;
    handleMoveOrAttack: (direction: MoveDirection) => Promise<void>;
    movePlayer: (x: number, y: number) => Promise<PlayerState | null>;
    fightMonster: (cellX: number, cellY: number) => Promise<void>;
    openBarrel: (cellX: number, cellY: number) => Promise<any>;
    collectResource: (cellX: number, cellY: number) => Promise<void>;
} {
    const dispatch = useDispatch();

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
                if (!token) return null;
                const url = `http://localhost:8001/game/${instanceId}/player/${myPlayerId}/move`;
                const body = { new_pos_x: newPosX, new_pos_y: newPosY };
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        Authorization: `Bearer ${token}`,
                    },
                    body: JSON.stringify(body),
                });
                if (!response.ok) return null;
                const updatedPlayer = await response.json();
                return updatedPlayer as PlayerState;
            } catch {
                return null;
            }
        },
        [instanceId, myPlayerId],
    );

    const fightMonster = useCallback(
        async (cellX: number, cellY: number) => {
            const targetCell = state.grid.find(
                (c) => c.x === cellX && c.y === cellY,
            );
            if (!targetCell?.monster) return;
            const token = user?.token;
            if (!token || !myPlayerId) return;
            await fetch("http://localhost:8001/game/attack", {
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
        },
        [user, state.grid, instanceId, myPlayerId],
    );

    const openBarrel = useCallback(
        async (cellX: number, cellY: number) => {
            const stored = localStorage.getItem("user");
            const token = stored ? JSON.parse(stored).token : "";
            if (!token) return null;
            const res = await fetch("http://localhost:8001/game/openBarrel", {
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
            if (!res.ok) return null;
            const data = await res.json();
            if (!data.updatedCell || !data.updatedPlayer) return null;
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
            };
            const mappedPlayer: PlayerState = data.updatedPlayer as PlayerState;
            dispatch(updateCell({ instanceId, updatedCell: mappedCell }));
            dispatch(updatePlayer({ instanceId, player: mappedPlayer }));
            return { updatedCell: mappedCell, updatedPlayer: mappedPlayer };
        },
        [instanceId, myPlayerId, dispatch],
    );

    const collectResource = useCallback(
        async (cellX: number, cellY: number) => {
            if (!instanceId || !user) return;
            const token = user.token;
            if (!token) return;
            await fetch("http://localhost:8001/game/collectResource", {
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
        },
        [instanceId, user, myPlayerId],
    );

    const handleMoveOrAttack = useCallback(
        async (direction: MoveDirection) => {
            if (!myPlayer || !isMyTurn) return;
            const { x: dx, y: dy } = DELTAS[direction];
            const targetX = myPlayer.position.x + dx;
            const targetY = myPlayer.position.y + dy;
            const targetCell = state.grid.find(
                (c) => c.x === targetX && c.y === targetY,
            );
            if (!targetCell) return;
            if (targetCell.monster) {
                await fightMonster(targetCell.x, targetCell.y);
                return;
            }
            await movePlayer(targetX, targetY);
        },
        [myPlayer, isMyTurn, state.grid, movePlayer, fightMonster],
    );

    return {
        myPlayer,
        isMyTurn,
        handleMoveOrAttack,
        movePlayer,
        fightMonster,
        openBarrel,
        collectResource,
    };
}
