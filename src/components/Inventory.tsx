//inventory.tsx

"use client";

import React, { useState } from "react";
import "../styles/inventory.css";

type InventoryItem = {
  count: number;
  description: string;
};

type InventoryProps = {
  items: Record<string, InventoryItem>;
  onUseItem: (type: string) => void;
};

export default function Inventory({ items, onUseItem }: InventoryProps) {
  const [filter, setFilter] = useState("");

  const handleFilterChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setFilter(e.target.value.toLowerCase());
  };

  const filteredItems = Object.entries(items)
    .filter(([key]) => key.toLowerCase() !== "barrbel") // Исключаем "бочку"
    .filter(([key]) => key.toLowerCase().includes(filter)); // Применяем поиск

  // Функция для получения пути к изображению ресурса
  const getResourceImagePath = (resourceType: string): string => {
    return `/main_resources/${resourceType}.webp`; // Формируем путь
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
        {filteredItems.map(([key, { count, description }]) => (
          <div className="inventory-item" key={key} onClick={() => onUseItem(key)}>
            <img
              src={getResourceImagePath(key)} // Динамически формируем путь к картинке
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
