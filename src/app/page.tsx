"use client";

import React, { useState } from "react";
import { GameProvider } from "../components/GameContext";
import GameManager from "../components/GameManager";

export default function Home() {
  const [inventoryOpen, setInventoryOpen] = useState(false);

  return (
    <GameProvider>
      <div>
        <button onClick={() => setInventoryOpen(!inventoryOpen)}>
          {inventoryOpen ? "Закрыть инвентарь" : "Открыть инвентарь"}
        </button>
        <GameManager inventoryOpen={inventoryOpen} />
      </div>
    </GameProvider>
  );
}
