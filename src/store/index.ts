
//===============================
// src/store/index.ts
//===============================


import { configureStore } from "@reduxjs/toolkit";
import gameReducer from "./slices/gameSlice"; // и другие редьюсеры при необходимости
import combatPresentationReducer from "./slices/combatPresentationSlice";

export const store = configureStore({
  reducer: {
    game: gameReducer,
    combatPresentation: combatPresentationReducer,
    // другие редьюсеры
  },
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
