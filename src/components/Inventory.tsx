"use client";

import React, { useState } from "react";
import "../styles/inventory.css";

type InventoryProps = {
  items: Record<string, { count: number; image: string; description: string }>;
  onClose: () => void;
};

export default function Inventory({ items, onClose }: InventoryProps) {
  const [filter, setFilter] = useState("");

  const handleFilterChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setFilter(e.target.value.toLowerCase());
  };

  const filteredItems = Object.entries(items).filter(([key]) =>
    key.toLowerCase().includes(filter)
  );

  return (
    <div className="inventory">
      <button className="inventory-close-button" onClick={onClose}>
        Закрыть
      </button>
      <input
        className="inventory-search"
        type="text"
        placeholder="Поиск предметов..."
        value={filter}
        onChange={handleFilterChange}
      />
      <div className="inventory-grid">
        {filteredItems.map(([key, { count, image, description }]) => (
          <div className="inventory-item" key={key}>
            <img src={image} alt={key} className="inventory-image" />
            <p>{key}</p>
            <p>Количество: {count}</p>
            <p className="inventory-description">{description}</p>
          </div>
        ))}
      </div>
    </div>
  );
}
