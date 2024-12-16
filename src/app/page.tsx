// src/components/Page.tsx

"use client";

import React from "react";
import { GameProvider, useGameContext } from "../components/GameContext";
import GameManager from "../components/GameManager";
import { useSearchParams } from "next/navigation";

export default function Page() {
  const searchParams = useSearchParams();
  const instanceId = searchParams.get("instanceId") || "test_instance";

  return (
    <GameProvider instanceId={instanceId}>
      <GameManager />
    </GameProvider>
  );
}
