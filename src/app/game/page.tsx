
//=======================
// src/app/game/page.tsx
//=======================

"use client";

import React from "react";
import { useSearchParams } from "next/navigation";
import { GameProvider } from "../../contexts/GameContextt";
import GameController from "../../components/GameController";

export default function GamePage() {
  const searchParams = useSearchParams();
  const instanceId = searchParams.get("instance_id");

  if (!instanceId) {
    return <div>Не задан instance_id</div>;
  }

  return (
    <GameProvider instanceId={instanceId}>
      <GameController />
    </GameProvider>
  );
}
