// src/components/Inventory.tsx

"use client";

import React, { useState } from "react";
import "../styles/inventory.css";
import { useGameContext } from "./GameContext";
import { Action } from "../logic/actions";

// Тип для элемента инвентаря
type InventoryItem = {
  count: number; // Количество предметов
  description: string; // Описание предмета
  image: string; // Путь к изображению
};

// Тип для свойств компонента инвентаря
type InventoryProps = {
  items: Record<string, InventoryItem>; // Список предметов инвентаря
  // onUseItem: (type: string) => void; // Удалено, теперь используется dispatch
};

// Компонент для отображения и управления инвентарем
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
    dispatch({ type: 'USE_ITEM', payload: { playerId: state.players[state.currentPlayerIndex].id, itemType: type } });
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
