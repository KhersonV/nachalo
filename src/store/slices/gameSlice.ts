import { createSlice, PayloadAction } from "@reduxjs/toolkit";
import type {
    GameState,
    PlayerState,
    Inventory,
    Cell,
} from "../../types/GameTypes";

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
            if (action.payload.grid !== undefined)
                state.grid = action.payload.grid;
            if (action.payload.mapWidth !== undefined)
                state.mapWidth = action.payload.mapWidth;
            if (action.payload.mapHeight !== undefined)
                state.mapHeight = action.payload.mapHeight;
            if (action.payload.players !== undefined)
                state.players = action.payload.players;
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
            console.log("[gameSlice/setMatchData] payload:", action.payload);
            console.log("[gameSlice/setMatchData] new state:", {
                players: state.players,
                active_user: state.active_user,
                instanceId: state.instanceId,
                isMapLoaded: state.isMapLoaded,
            });
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

        combatExchange(
            state,
            action: PayloadAction<{
                instanceId: string;
                attacker: { id: number; new_hp: number };
                target: { id: number; new_hp: number };
            }>,
        ) {
            if (action.payload.instanceId !== state.instanceId) return;
            for (const p of state.players) {
                if (p.user_id === action.payload.attacker.id)
                    p.health = action.payload.attacker.new_hp;
                if (p.user_id === action.payload.target.id)
                    p.health = action.payload.target.new_hp;
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
            if (i >= 0) state.players[i] = updated;
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

            const index = state.grid.findIndex(
                (cell) => cell.x === uc.x && cell.y === uc.y,
            );

            if (index !== -1) {
                state.grid[index] = uc;
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
        setQuestFoundNotification(state, action: PayloadAction<string | null>) {
            state.questFoundNotification = action.payload;
        },
    },
});

export const {
    setMatchData,
    movePlayer,
    combatExchange,
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
} = gameSlice.actions;

export default gameSlice.reducer;
