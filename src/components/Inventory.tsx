// src/components/Inventory.tsx

"use client";

import React, { useState } from "react";
import "../styles/inventory.css";
import { useGameContext } from "./GameContext";

type InventoryItem = {
  count: number;
  description: string;
  image: string;
};

type InventoryProps = {
  items: Record<string, InventoryItem>;
};

export default function Inventory({ items }: InventoryProps) {
  const [filter, setFilter] = useState("");
  const { dispatch, state } = useGameContext();

  const handleFilterChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setFilter(e.target.value.toLowerCase());
  };

  const filteredItems = Object.entries(items)
    .filter(([key]) => key.toLowerCase() !== "barrbel")
    .filter(([key]) => key.toLowerCase().includes(filter));

  const getResourceImagePath = (resourceType: string): string => {
    return `/main_resources/${resourceType}.webp`;
  };

  const handleUseItem = (type: string) => {
    const currentPlayer = state.players[state.currentPlayerIndex];
    if (!currentPlayer) return;
  
    const item = currentPlayer.inventory[type];
    if (!item || item.count <= 0) {
      console.error("Ресурс отсутствует или закончился.");
      return;
    }
  
    dispatch({
      type: "USE_ITEM",
      payload: { playerId: currentPlayer.id, itemType: type },
    });
  };

  return (
    <div className="inventory">
      <input
        className="inventory-search"
        type="text"
        placeholder="Поиск предметов..."
        value={filter}
        onChange={handleFilterChange}
      />

      <div className="inventory-grid">
        {filteredItems.map(([key, { count, description, image }]) => (
          <div
            className="inventory-item"
            key={key}
            onClick={() => handleUseItem(key)}
          >
            <img
              src={getResourceImagePath(key)}
              alt={key}
              className="inventory-image"
            />
            <p>{key}</p>
            <p>Количество: {count}</p>
            <p className="inventory-description">{description}</p>
            <p className="inventory-action-hint">Нажмите для использования</p>
          </div>
        ))}
      </div>
    </div>
  );
}
