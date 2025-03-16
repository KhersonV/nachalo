//==================================
// src/components/Inventory.tsx
//==================================

"use client";

import React from "react";
import { useGame } from "../contexts/GameContextt";
import { useAuth } from "../contexts/AuthContext";
import type { Inventory, InventoryItem } from "../types/GameTypes";

const Inventory: React.FC = () => {
  const { state } = useGame();
  const { user } = useAuth();

  console.log("[Inventory] state.players:", state.players);
  console.log("[Inventory] user:", user);

  // Выведем данные каждого игрока для отладки
  state.players.forEach((p, index) => {
    console.log(`[Inventory] Player[${index}]: id=${p.user_id}, user_id=${p.user_id}`);
  });

  // Ищем игрока по user.id (сравниваем числовые значения)
  const player = state.players.find((p) => p.user_id === user?.id);


  console.log("[Inventory] Найден player:", player);

  if (!player) {
    return <div>Инвентарь недоступен</div>;
  }

  // Пробуем распарсить инвентарь
  let inventory: Inventory;
  try {
    const parsed =
      typeof player.inventory === "string"
        ? JSON.parse(player.inventory)
        : player.inventory;
    // Если объект не содержит ключей resources и artifacts, оборачиваем его в нужную структуру.
    if (!("resources" in parsed) || !("artifacts" in parsed)) {
      inventory = { resources: parsed, artifacts: {} };
    } else {
      inventory = parsed;
    }
    console.log("[Inventory] Parsed inventory:", inventory);
  } catch (e) {
    console.error("Ошибка парсинга инвентаря:", e);
    inventory = { resources: {}, artifacts: {} };
  }

  // Функция для рендера элементов инвентаря (для любого раздела)
  const renderItems = (items: Record<string, InventoryItem> | undefined) => {
    const safeItems = items ?? {};
    return Object.entries(safeItems)
      .filter(([, item]) => item.count > 0)
      .map(([key, item]) => (
        <div
          key={key}
          style={{
            border: "1px solid #ccc",
            margin: "4px",
            padding: "4px",
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
          }}
        >
          <img
            src={item.image}
            alt={item.name || key}
            style={{ width: "40px", height: "40px", objectFit: "cover" }}
          />
          <div>{item.name || key}</div>
          <div>Кол-во: {item.count}</div>
        </div>
      ));
  };

  return (
    <div
      style={{
        position: "absolute",
        top: "10%",
        right: "10%",
        backgroundColor: "rgba(0, 0, 0, 0.8)",
        color: "#fff",
        padding: "20px",
        zIndex: 100,
        borderRadius: "8px",
        maxHeight: "80vh",
        overflowY: "auto",
      }}
    >
      <h2>Инвентарь</h2>
      <div style={{ display: "flex", flexWrap: "wrap" }}>
        <div style={{ marginRight: "20px" }}>
          <h3>Ресурсы</h3>
          {renderItems(inventory.resources)}
        </div>
        <div>
          <h3>Артефакты</h3>
          {renderItems(inventory.artifacts)}
        </div>
      </div>
    </div>
  );
};

export default Inventory;
