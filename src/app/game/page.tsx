// app/game/page.tsx
"use client";

import React from "react";
import { useSearchParams } from "next/navigation";
import { GameProvider } from "../../contexts/GameContextt";
import MapWithCamera from "../../components/MapWithCamera";

export default function GamePage() {
  const searchParams = useSearchParams();
  const instanceId = searchParams.get("instance_id"); // без подчёркивания

  if (!instanceId) {
    return <div>Не задан instance_id</div>;
  }

  return (
    <GameProvider instanceId={instanceId}>
      <MapWithCamera tileSize={80} viewportWidth={800} viewportHeight={600} />
    </GameProvider>
  );
}
