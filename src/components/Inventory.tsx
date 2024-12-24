// src/components/Inventory.tsx

"use client";

import React, { useState } from "react";
import "../styles/inventory.css";
import { useGameContext } from "./GameContext";

type InventoryItem = {
  count: number;
  description: string;
  image: string;
  bonus?: Record<string, number>;
};

type InventoryProps = {
  // Теперь ожидаем два набора предметов:
  resources: Record<string, InventoryItem>;
  artifacts: Record<string, InventoryItem>;
};

export default function Inventory({ resources, artifacts }: InventoryProps) {
  const { dispatch, state } = useGameContext();
  const [activeTab, setActiveTab] = useState<"resources" | "artifacts">("resources");
  const [filter, setFilter] = useState("");

  const handleFilterChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setFilter(e.target.value.toLowerCase());
  };

  // Фильтрация
  const filteredResources = Object.entries(resources).filter(([key]) =>
    key.toLowerCase().includes(filter)
  );
  const filteredArtifacts = Object.entries(artifacts).filter(([key]) =>
    key.toLowerCase().includes(filter)
  );

  const handleUseItem = (type: string) => {
    const currentPlayer = state.players[state.currentPlayerIndex];
    if (!currentPlayer) return;

    // Пытаемся найти предмет среди resources
    const item = currentPlayer.inventory.resources[type];
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
      <div className="inventory-tabs">
        <button
          className={activeTab === "resources" ? "active-tab" : ""}
          onClick={() => setActiveTab("resources")}
        >
          Ресурсы
        </button>
        <button
          className={activeTab === "artifacts" ? "active-tab" : ""}
          onClick={() => setActiveTab("artifacts")}
        >
          Артефакты
        </button>
      </div>

      <input
        className="inventory-search"
        type="text"
        placeholder="Поиск предметов..."
        value={filter}
        onChange={handleFilterChange}
      />

      {activeTab === "resources" && (
        <div className="inventory-grid">
          {filteredResources.map(([key, item]) => (
            <div 
              className="inventory-item" 
              key={key}
              onClick={() => handleUseItem(key)}
            >
              <img
                src={item.image || `/main_resources/${key}.webp`} 
                alt={key}
                className="inventory-image"
              />
              <p>{key}</p>
              <p>Количество: {item.count}</p>
              <p className="inventory-description">{item.description}</p>
              <p className="inventory-action-hint">Нажмите для использования</p>
            </div>
          ))}
        </div>
      )}

{activeTab === "artifacts" && (
  <div className="inventory-grid">
    {filteredArtifacts.map(([key, item]) => (
      <div className="inventory-item" key={key}>
        <img
          src={item.image} 
          alt={key}
          className="inventory-image"
        />
        <p>{key}</p>
        <p>Количество: {item.count}</p>
        <p className="inventory-description">{item.description}</p>
        
        {/* Отображение бонусов */}
        {item.bonus && (
          <div className="inventory-bonus">
            <p><strong>Бонусы:</strong></p>
            <ul>
              {Object.entries(item.bonus).map(([attribute, value]) => (
                <li key={attribute}>
                  {attribute}: +{value}
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    ))}
  </div>
)}
    </div>
  );
}
