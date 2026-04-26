import { createSlice, PayloadAction } from "@reduxjs/toolkit";
import type {
    ActionLogEntry,
    ActionLogEntryInput,
    GameState,
    PlayerState,
    Inventory,
    Cell,
} from "../../types";
import type { CombatExchangePayload, CombatTargetRef } from "@/types/combat";

// Fast lookup index for cells by "x:y" → index in state.grid.
// Kept at module level for minimal changes (O(1) updates).

const ACTION_LOG_LIMIT = 10;
const ACTION_LOG_DEDUPE_MS = 1400;
let actionLogSeq = 0;

export type QuestNotificationType =
    | "QUEST_ARTIFACT_FOUND"
    | "PLAYER_LEFT_PORTAL"
    | "MY_PLAYER_DEFEATED";

export interface QuestFoundNotification {
    eventType: QuestNotificationType;
    message: string;
    instanceId?: string;
    playerName?: string;
    x?: number;
    y?: number;
    userId?: number;
}

let gridIndex: Record<string, number> = {};

function prepareActionLogEntry(input: ActionLogEntryInput): ActionLogEntry {
    const timestamp = input.timestamp ?? Date.now();

    return {
        ...input,
        id: input.id ?? `${timestamp}-${actionLogSeq++}`,
        timestamp,
    };
}

function appendActionLogEntry(state: GameState, entry: ActionLogEntry) {
    if (!entry.message.trim()) return;

    if (
        entry.dedupeKey &&
        state.actionLog.some((item) => item.dedupeKey === entry.dedupeKey)
    ) {
        return;
    }

    const previous = state.actionLog[state.actionLog.length - 1];
    if (
        previous &&
        previous.message === entry.message &&
        entry.timestamp - previous.timestamp < ACTION_LOG_DEDUPE_MS
    ) {
        return;
    }

    state.actionLog.push(entry);
    if (state.actionLog.length > ACTION_LOG_LIMIT) {
        state.actionLog = state.actionLog.slice(-ACTION_LOG_LIMIT);
    }
}

function applyActorHp(
    state: GameState,
    target: CombatTargetRef,
    hpAfter: number,
) {
    if (target.type === "player") {
        const player = state.players.find((p) => p.user_id === target.id);
        if (player) player.health = hpAfter;
        return;
    }

    const cell = state.grid.find(
        (item) =>
            item.monster?.db_instance_id === target.id ||
            item.monster?.id === target.id,
    );
    if (cell?.monster) {
        cell.monster.health = hpAfter;
    }
}

function coerceInteger(value: unknown): number | undefined {
    if (typeof value === "number" && Number.isFinite(value)) {
        return Math.trunc(value);
    }

    if (typeof value === "string" && value.trim() !== "") {
        const parsed = Number(value);
        if (Number.isFinite(parsed)) {
            return Math.trunc(parsed);
        }
    }

    return undefined;
}

function normalizeGridCells(
    grid: Cell[] | undefined,
    mapWidth: number | undefined,
    mapHeight: number | undefined,
): Cell[] {
    if (!Array.isArray(grid) || grid.length === 0) {
        return [];
    }

    if (!mapWidth || !mapHeight) {
        return grid;
    }

    const expectedSize = mapWidth * mapHeight;
    if (grid.length !== expectedSize) {
        return grid;
    }

    const dense: Array<Cell | undefined> = new Array(expectedSize);

    for (const cell of grid) {
        const x = coerceInteger((cell as any)?.x);
        const y = coerceInteger((cell as any)?.y);

        if (
            x === undefined ||
            y === undefined ||
            x < 0 ||
            y < 0 ||
            x >= mapWidth ||
            y >= mapHeight
        ) {
            return grid;
        }

        const index = y * mapWidth + x;
        if (dense[index] !== undefined) {
            return grid;
        }

        dense[index] = cell;
    }

    if (dense.some((cell) => cell === undefined)) {
        return grid;
    }

    return dense as Cell[];
}

function normalizePlayerState(
    player: PlayerState,
    existing?: PlayerState,
): PlayerState {
    const incoming = (player ?? {}) as any;
    const merged = { ...existing, ...incoming } as any;

    const nextUserID = coerceInteger(incoming.user_id ?? incoming.player_id);
    if (nextUserID !== undefined) {
        merged.user_id = nextUserID;
    }

    const nextPosX = coerceInteger(incoming?.position?.x);
    const nextPosY = coerceInteger(incoming?.position?.y);
    const hasExistingPosition =
        existing?.position &&
        Number.isFinite(existing.position.x) &&
        Number.isFinite(existing.position.y);

    // Some non-match handlers can return a persistent player snapshot with the
    // zero-value position (0,0). Keep the match position in that case.
    const looksLikeUninitializedMatchPosition =
        nextPosX === 0 &&
        nextPosY === 0 &&
        hasExistingPosition &&
        (existing.position.x !== 0 || existing.position.y !== 0);

    if (
        nextPosX !== undefined &&
        nextPosY !== undefined &&
        !looksLikeUninitializedMatchPosition
    ) {
        merged.position = { x: nextPosX, y: nextPosY };
    } else if (hasExistingPosition) {
        merged.position = { ...existing.position };
    } else {
        merged.position = {
            x: nextPosX ?? 0,
            y: nextPosY ?? 0,
        };
    }

    const nextSightRange =
        coerceInteger(incoming.sightRange) ??
        coerceInteger(incoming.visionRange) ??
        coerceInteger(incoming.vision) ??
        existing?.sightRange ??
        2;
    merged.sightRange = Math.max(0, nextSightRange);

    const nextAttackRange =
        coerceInteger(incoming.attackRange) ?? existing?.attackRange ?? 1;
    merged.attackRange = Math.max(1, nextAttackRange);

    const nextGroupID = coerceInteger(incoming.group_id);
    if (nextGroupID !== undefined) {
        const looksLikePersistentSnapshot =
            nextGroupID === 0 &&
            looksLikeUninitializedMatchPosition &&
            (existing?.group_id ?? 0) > 0;

        if (!looksLikePersistentSnapshot) {
            merged.group_id = nextGroupID;
        }
    } else if (existing?.group_id !== undefined) {
        merged.group_id = existing.group_id;
    }

    return merged as PlayerState;
}

function rebuildGridIndex(cells: Cell[]) {
    gridIndex = {};

    for (let i = 0; i < cells.length; i++) {
        const cell = cells[i] as Cell | undefined;
        if (
            cell &&
            typeof cell.x === "number" &&
            typeof cell.y === "number"
        ) {
            gridIndex[`${cell.x}:${cell.y}`] = i;
        }
    }
}

const initialState: GameState = {
    instanceId: "",
    mode: "",
    grid: [],
    mapWidth: 0,
    mapHeight: 0,
    players: [],
    active_user: 0,
    turnNumber: 1,
    isMapLoaded: false,
    questArtifactId: 0,
    questArtifactName: "",
    questArtifactImage: "",
    questArtifactDescription: "",
    questFoundNotification: null,
    actionLog: [],
};

const gameSlice = createSlice({
    name: "game",
    initialState,
    reducers: {
        setMatchData(state, action: PayloadAction<Partial<GameState>>) {
            // Обновлять только те поля, которые реально пришли в payload,
            // и не трогать grid/mapWidth/mapHeight если они undefined.

            if (action.payload.instanceId !== undefined)
                state.instanceId = action.payload.instanceId;
            if (action.payload.mode !== undefined)
                state.mode = action.payload.mode;
            if (action.payload.mapWidth !== undefined)
                state.mapWidth = action.payload.mapWidth;
            if (action.payload.mapHeight !== undefined)
                state.mapHeight = action.payload.mapHeight;
            if (action.payload.grid !== undefined) {
                state.grid = normalizeGridCells(
                    action.payload.grid,
                    state.mapWidth,
                    state.mapHeight,
                );
            }
            if (action.payload.players !== undefined) {
                const existingByUserId = new Map(
                    state.players.map((player) => [player.user_id, player]),
                );
                state.players = action.payload.players.map((player) =>
                    normalizePlayerState(
                        player,
                        existingByUserId.get(player.user_id),
                    ),
                );
            }
            if (action.payload.active_user !== undefined)
                state.active_user = action.payload.active_user;
            if (action.payload.turnNumber !== undefined)
                state.turnNumber = action.payload.turnNumber;
            if (action.payload.questArtifactId !== undefined)
                state.questArtifactId = action.payload.questArtifactId;
            if (action.payload.questArtifactName !== undefined)
                state.questArtifactName = action.payload.questArtifactName;
            if (action.payload.questArtifactImage !== undefined)
                state.questArtifactImage = action.payload.questArtifactImage;
            if (action.payload.questArtifactDescription !== undefined)
                state.questArtifactDescription =
                    action.payload.questArtifactDescription;
            // Ставим isMapLoaded только если карта уже есть
            if (state.grid.length > 0 && state.mapWidth && state.mapHeight) {
                state.isMapLoaded = true;
            }
            rebuildGridIndex(state.grid);
        },

        movePlayer(
            state,
            action: PayloadAction<{
                instanceId: string;
                userId: number;
                newPosition: { x: number; y: number };
            }>,
        ) {
            if (action.payload.instanceId !== state.instanceId) return;
            const p = state.players.find(
                (p) => p.user_id === action.payload.userId,
            );
            if (p) p.position = action.payload.newPosition;
        },

        applyCombatExchangeState(
            state,
            action: PayloadAction<CombatExchangePayload>,
        ) {
            if (action.payload.instanceId !== state.instanceId) return;

            for (const step of action.payload.steps) {
                if (step.kind === "death") continue;
                applyActorHp(state, step.target, step.targetHpAfter);
            }
        },

        updateInventory(
            state,
            action: PayloadAction<{
                instanceId: string;
                userId: number;
                inventory: Inventory;
            }>,
        ) {
            if (action.payload.instanceId !== state.instanceId) return;
            const p = state.players.find(
                (p) => p.user_id === action.payload.userId,
            );
            if (p) p.inventory = action.payload.inventory;
        },

        updatePlayer(
            state,
            action: PayloadAction<{
                instanceId: string;
                player: PlayerState;
            }>,
        ) {
            if (action.payload.instanceId !== state.instanceId) return;
            const updated = action.payload.player;
            const i = state.players.findIndex(
                (p) => p.user_id === updated.user_id,
            );
            if (i >= 0) {
                state.players[i] = normalizePlayerState(updated, state.players[i]);
            }
        },

        updateCell(
            state,
            action: PayloadAction<{
                instanceId: string;
                updatedCell: Cell;
            }>,
        ) {
            if (action.payload.instanceId !== state.instanceId) return;

            const uc = action.payload.updatedCell;

            const key = `${uc.x}:${uc.y}`;
            let index = gridIndex[key];
            if (
                index === undefined ||
                index === null ||
                index < 0 ||
                index >= state.grid.length
            ) {
                // Fallback to findIndex if index missing/out-of-sync
                index = state.grid.findIndex(
                    (cell) => cell.x === uc.x && cell.y === uc.y,
                );
            }

            if (index !== -1) {
                // Merge incoming updatedCell into existing cell but
                // ensure destroyed/removed structures are cleared so
                // they don't persist in UI when backend sends partial
                // updates or health <= 0.
                const existing = state.grid[index];
                const merged: any = { ...existing, ...uc };

                // If backend explicitly cleared structure_type (empty string/null)
                // or reported structure_health <= 0 — remove structure fields.
                const structType = (uc as any).structure_type;
                const structHealth = (uc as any).structure_health;
                if (
                    structType === "" ||
                    structType === null ||
                    (typeof structHealth === "number" && structHealth <= 0)
                ) {
                    merged.structure_type = undefined;
                    merged.structure_owner_user_id = undefined;
                    merged.structure_health = undefined;
                    merged.structure_defense = undefined;
                    merged.structure_attack = undefined;
                    merged.is_under_construction = false;
                    merged.construction_turns_left = undefined;
                }
                // Ensure a helpful image path is available for structures so
                // various UI components can render a built structure image
                // instead of a generic placeholder.
                const sType = merged.structure_type as string | undefined;
                if (sType) {
                    switch (sType) {
                        case "scout_tower":
                            merged.structure_image =
                                "/Forge-items/scout_tower.png";
                            break;
                        case "turret":
                            merged.structure_image = "/Forge-items/turret.png";
                            break;
                        case "wall":
                            merged.structure_image = "/Forge-items/wall.png";
                            break;
                        default:
                            merged.structure_image = undefined;
                    }
                } else {
                    merged.structure_image = undefined;
                }

                state.grid[index] = merged;
                // maintain index
                gridIndex[key] = index;
            } else {
                // New cell — append and index it
                state.grid.push(uc as any);
                gridIndex[key] = state.grid.length - 1;
            }
        },

        setActiveUser(
            state,
            action: PayloadAction<{
                instanceId: string;
                active_user: number;
                turnNumber: number;
                energy?: number;
            }>,
        ) {
            if (action.payload.instanceId !== state.instanceId) return;
            state.active_user = action.payload.active_user;
            state.turnNumber = action.payload.turnNumber;
            const p = state.players.find(
                (p) => p.user_id === action.payload.active_user,
            );
            if (p && action.payload.energy !== undefined)
                p.energy = action.payload.energy;
        },

        updatePlayerPosition: (state, action) => {
            const { userId, newPosition } = action.payload;
            const player = state.players.find((p) => p.user_id === userId);
            if (player) {
                player.position = newPosition;
            }
        },

        updatePlayerHealth: (state, action) => {
            const { userId, hp } = action.payload;
            // Находим игрока в текущем матче
            const player = state.players.find((p) => p.user_id === userId);
            if (player) {
                player.health = hp;
            }
        },

        playerDefeated(
            state,
            action: PayloadAction<{
                instanceId: string;
                userId: number;
            }>,
        ) {
            if (action.payload.instanceId !== state.instanceId) return;
            state.players = state.players.filter(
                (p) => p.user_id !== action.payload.userId,
            );
        },

        turnPassed(
            state,
            action: PayloadAction<{
                instanceId: string;
                active_user: number;
            }>,
        ) {
            if (action.payload.instanceId !== state.instanceId) return;
            state.active_user = action.payload.active_user;
        },

        resetState() {
            return initialState;
        },
        setInstanceId(state, action: PayloadAction<string>) {
            state.instanceId = action.payload;
        },
        setQuestFoundNotification(
            state,
            action: PayloadAction<QuestFoundNotification | null>,
        ) {
            state.questFoundNotification = action.payload;
        },
        addActionLogEntry: {
            reducer(state, action: PayloadAction<ActionLogEntry>) {
                appendActionLogEntry(state, action.payload);
            },
            prepare(input: ActionLogEntryInput) {
                return { payload: prepareActionLogEntry(input) };
            },
        },
        addActionLogEntries: {
            reducer(state, action: PayloadAction<ActionLogEntry[]>) {
                for (const entry of action.payload) {
                    appendActionLogEntry(state, entry);
                }
            },
            prepare(inputs: ActionLogEntryInput[]) {
                return {
                    payload: inputs.map((input, index) =>
                        prepareActionLogEntry({
                            ...input,
                            timestamp: input.timestamp ?? Date.now() + index,
                        }),
                    ),
                };
            },
        },
        clearActionLog(state) {
            state.actionLog = [];
        },
    },
});

export const {
    setMatchData,
    movePlayer,
    applyCombatExchangeState,
    updateInventory,
    updatePlayer,
    updateCell,
    setActiveUser,
    playerDefeated,
    turnPassed,
    resetState,
    setInstanceId,
    updatePlayerPosition,
    updatePlayerHealth,
    setQuestFoundNotification,
    addActionLogEntry,
    addActionLogEntries,
    clearActionLog,
} = gameSlice.actions;

export default gameSlice.reducer;
