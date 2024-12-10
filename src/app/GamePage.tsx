"use client";

import React, { useState } from "react";
import { useSearchParams } from "next/navigation";
import { GameProvider } from "../components/GameContext";
import GameManager from "../components/GameManager";

export default function Page() {
  const searchParams = useSearchParams();
  const instanceId = searchParams.get("instanceId") || "test_instance"; // Если нет, используем тестовый ID

  const [inventoryOpen, setInventoryOpen] = useState(false);

  return (
    <GameProvider instanceId={instanceId}>
      <div style={{ padding: '10px' }}>
        <button onClick={() => setInventoryOpen(!inventoryOpen)}>
          {inventoryOpen ? "Закрыть инвентарь" : "Открыть инвентарь"}
        </button>
        <GameManager inventoryOpen={inventoryOpen} setInventoryOpen={setInventoryOpen} />
      </div>
    </GameProvider>
  );
}
