//GamePage.tsx

"use client";

import React from "react";
import { GameProvider } from "../components/GameContext";
import GameManager from "../components/GameManager";

/**
 * Главная страница игры. Здесь инициализируется контекст игры через GameProvider
 * и подключается компонент GameManager, который управляет всем игровым процессом.
 */
export default function GamePage({ params }: { params: { instanceId: string } }) {
  console.log(`[Шаг 1] [Файл: GamePage.tsx] - Загружаем страницу игры с instanceId=${params.instanceId}.`);

  return (
    <GameProvider instanceId={params.instanceId}>
      <GameManager  />
    </GameProvider>
  );
}
