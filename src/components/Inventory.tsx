//inventory.tsx

"use client";

import React, { useState } from "react";
import "../styles/inventory.css";

type InventoryItem = {
  count: number;
  image: string;
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

  const filteredItems = Object.entries(items).filter(([key]) =>
    key.toLowerCase().includes(filter)
  );

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
        {filteredItems.map(([key, { count, image, description }]) => (
          <div className="inventory-item" key={key} onClick={() => onUseItem(key)}>
            <img src={image} alt={key} className="inventory-image" />
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
