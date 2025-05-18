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
  const { state, dispatch } = useGame();
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

  // Обработчик использования предмета: отправляем запрос на сервер
  const handleUseItem = async (
    section: "resources" | "artifacts",
    key: string,
    item: InventoryItem
  ) => {
    if (item.count <= 0 || !user) return;

    console.log(`Отправляем запрос на использование предмета: ${item.name || key}`);

    // Здесь предполагается, что key имеет формат "itemType_itemID".
    // Разбиваем ключ для получения типа и идентификатора.
    const [itemType, idStr] = key.split("_");
    const itemID = parseInt(idStr, 10);

    try {
      const response = await fetch(`http://localhost:8001/game/player/${user.id}/inventory/use`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          item_type: itemType,
          item_id: itemID,
          count: 1, // можно изменить, если нужно использовать более одного предмета
        }),
      });

      if (!response.ok) {
        console.error("Ошибка использования предмета", await response.text());
        return;
      }

      console.log("Предмет успешно использован");
      
      // Если сервер возвращает обновлённый инвентарь или информацию об игроке,
      // можно обновить состояние через dispatch или иным способом.
      // Например, вызвать fetch для обновления данных игрока.
      
    } catch (error) {
      console.error("Ошибка запроса на использование предмета", error);
    }
  };

  // Функция для рендера элементов инвентаря (для любого раздела)
  const renderItems = (
    section: "resources" | "artifacts",
    items: Record<string, InventoryItem> | undefined
  ) => {
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
          <button
            className={styles.useButton}
            onClick={() => handleUseItem(section, key, item)}
          >
            Использовать
          </button>
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
            {renderItems("resources", inventory.resources)}
          </div>
        </div>
        <div className={styles.section}>
          <h3 className={styles.sectionTitle}>Артефакты</h3>
          <div className={styles.itemsContainer}>
            {renderItems("artifacts", inventory.artifacts)}
          </div>
        </div>
      </div>
    </div>
  );
};

export default Inventory;
