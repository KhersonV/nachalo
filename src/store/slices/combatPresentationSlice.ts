import { createSlice, PayloadAction } from "@reduxjs/toolkit";
import type {
    CombatPresentationState,
    QueuedCombatExchange,
} from "@/types/combat";

const initialState: CombatPresentationState = {
    queue: [],
    seenExchangeIds: {},
    activeExchangeId: null,
};

const combatPresentationSlice = createSlice({
    name: "combatPresentation",
    initialState,
    reducers: {
        enqueueCombatExchange(
            state,
            action: PayloadAction<QueuedCombatExchange>,
        ) {
            const exchange = action.payload;
            if (state.seenExchangeIds[exchange.exchangeId]) return;

            state.seenExchangeIds[exchange.exchangeId] = true;
            state.queue.push(exchange);
        },
        startCombatExchange(state, action: PayloadAction<string>) {
            state.activeExchangeId = action.payload;
        },
        finishCombatExchange(state, action: PayloadAction<string>) {
            if (state.activeExchangeId === action.payload) {
                state.activeExchangeId = null;
            }
            state.queue = state.queue.filter(
                (item) => item.exchangeId !== action.payload,
            );
        },
        clearCombatPresentation() {
            return initialState;
        },
    },
});

export const {
    enqueueCombatExchange,
    startCombatExchange,
    finishCombatExchange,
    clearCombatPresentation,
} = combatPresentationSlice.actions;

export default combatPresentationSlice.reducer;
