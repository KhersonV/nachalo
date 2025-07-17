
//===============================
// src/store/index.ts
//===============================


import { configureStore } from "@reduxjs/toolkit";
import gameReducer from "./slices/gameSlice"; // и другие редьюсеры при необходимости

export const store = configureStore({
  reducer: {
    game: gameReducer,
    // другие редьюсеры
  },
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
