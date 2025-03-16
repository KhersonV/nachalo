//==================================
// src/components/Inventory.tsx
//==================================

"use client";

import React from "react";
import { useGame } from "../contexts/GameContextt";
import { useAuth } from "../contexts/AuthContext";
import type { Inventory, InventoryItem } from "../types/GameTypes";
import styles from "../styles/Inventory.module.css";

const Inventory: React.FC = () => {
  const { state } = useGame();
  const { user } = useAuth();

  // Ищем игрока по user.id (сравниваем числовые значения)
  const player = state.players.find((p) => p.user_id === user?.id);

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
        <div key={key} className={styles.item}>
          <img
            src={item.image}
            alt={item.name || key}
            className={styles.itemImage}
          />
          <div className={styles.itemName}>{item.name || key}</div>
          <div className={styles.itemCount}>Кол-во: {item.count}</div>
        </div>
      ));
  };

  return (
    <div className={styles.inventoryContainer}>
      <h2 className={styles.title}>Инвентарь</h2>
      <div className={styles.sections}>
        <div className={styles.section}>
          <h3 className={styles.sectionTitle}>Ресурсы</h3>
          <div className={styles.itemsContainer}>
            {renderItems(inventory.resources)}
          </div>
        </div>
        <div className={styles.section}>
          <h3 className={styles.sectionTitle}>Артефакты</h3>
          <div className={styles.itemsContainer}>
            {renderItems(inventory.artifacts)}
          </div>
        </div>
      </div>
    </div>
  );
};

export default Inventory;
