"use client";

import React, { useState } from "react";
import Map from "../components/Map";
import Inventory from "../components/Inventory";

export default function Home() {
  const [inventoryOpen, setInventoryOpen] = useState(false);
  const [inventoryItems, setInventoryItems] = useState<Record<string, { count: number; image: string; description: string }>>({});

  const toggleInventory = () => setInventoryOpen(!inventoryOpen);

  return (
    <div>
      <button onClick={toggleInventory}>Открыть инвентарь</button>
      <Map updateInventory={setInventoryItems} />
      {inventoryOpen && <Inventory items={inventoryItems} onClose={toggleInventory} />}
    </div>
  );
}
