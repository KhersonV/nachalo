//==================================
// src/components/Inventory.tsx
//==================================

"use client";

import React, { useMemo } from "react";
import { useGame } from "../contexts/GameContextt";
import { useAuth } from "../contexts/AuthContext";
import type { RawInventoryItem, PlayerState } from "../types/GameTypes";
import styles from "../styles/Inventory.module.css";
const Inventory: React.FC = () => {
  const { state, dispatch} = useGame();
  const { user } = useAuth();
  const { instanceId } = state;

  // 1) Находим игрока
  const player = state.players.find((p) => p.user_id === user?.id);
  if (!player) return <div>Инвентарь недоступен</div>;

  // normalize-функция: из любого «сырого» объекта делаем RawInventoryItem
  const normalize = (it: any, keyHint?: string): RawInventoryItem => {
    // если keyHint вида "resource_2", распарсим его
    let hintType: "resource" | "artifact" | undefined;
    let hintId: number | undefined;
    if (keyHint) {
      const [t, id] = keyHint.split("_");
      if ((t === "resource" || t === "artifact") && !isNaN(+id)) {
        hintType = t;
        hintId = +id;
      }
    }

    const item_type = (it.item_type || it.type || hintType || "resource") as "resource" | "artifact";
    const item_id   = it.item_id || it.id || hintId || 0;
    const name      = it.name || it.item_name || item_type + "_" + item_id;
    const item_count= it.item_count || it.count || 1;
    const image     = it.image || it.image_url || "";
    const description = it.description || it.item_description || "";

    return { item_type, item_id, name, item_count, image, description, bonus: it.bonus, effect: it.effect };
  };

  // 2) Из разных форматов собираем единый плоский массив
  const rawItems: RawInventoryItem[] = useMemo(() => {
    let parsed: any;
    try {
      parsed = typeof player.inventory === "string"
        ? JSON.parse(player.inventory)
        : player.inventory;
    } catch {
      return [];
    }

    let flat: any[] = [];
    if (Array.isArray(parsed)) {
      flat = parsed;
    } else if (parsed.resources || parsed.artifacts) {
      flat = [
        ...(parsed.resources ? Object.entries(parsed.resources) : []),
        ...(parsed.artifacts ? Object.entries(parsed.artifacts) : []),
      ].map(
        ([key, val]) => ({ val, key }) // запомним ключ для normalize
      );
    } else {
      flat = Object.entries(parsed).map(([key, val]) => ({ val, key }));
    }

    return flat.map(entry => {
      // entry может быть либо raw объект, либо {val, key}
      if ((entry as any).val !== undefined) {
        return normalize((entry as any).val, (entry as any).key);
      } else {
        return normalize(entry);
      }
    });
  }, [player.inventory]);

  // 3) Группируем
  const { resources, artifacts } = useMemo(() => {
    return rawItems.reduce<{
      resources: Record<string, RawInventoryItem>;
      artifacts: Record<string, RawInventoryItem>;
    }>(
      (acc, it) => {
        const key = `${it.item_type}_${it.item_id}`;
        if (it.item_type === "resource") acc.resources[key] = it;
        else acc.artifacts[key] = it;
        return acc;
      },
      { resources: {}, artifacts: {} }
    );
  }, [rawItems]);
  
  // 4) Обработчик использования предмета
  const handleUseItem = async (
    section: "resources" | "artifacts",
    key: string,
    item: RawInventoryItem
  ) => {
    if (item.item_count <= 0 || !user) return;

    const [item_type, idStr] = key.split("_");
    const item_id = parseInt(idStr, 10);

    try {
      const res = await fetch(
        `http://localhost:8001/game/player/${user.id}/inventory/use`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ item_type, item_id, item_count: 1 }),
        }
      );
      if (!res.ok) {
        console.error("Ошибка использования предмета", await res.text());
      }
      const updatedPlayer = await res.json() as PlayerState;
      console.log("⚕️ after useItem, updatedPlayer.health =", updatedPlayer.health);
       dispatch({
        type: "UPDATE_PLAYER",
        payload: {
          instanceId,
          player: updatedPlayer,
        },
      });
    } catch (err) {
      console.error("Ошибка запроса на использование предмета", err);
    }
  };

  // 5) Рендер элементов
  const renderItems = (
    section: "resources" | "artifacts",
    items: Record<string, RawInventoryItem>
  ) =>
    Object.entries(items)
      .filter(([, item]) => item.item_count > 0)
      .map(([key, item]) => (
        <div key={key} className={styles.item}>
          <img
            src={item.image}
            alt={item.name}
            className={styles.itemImage}
            loading="lazy"
          />
          <div className={styles.itemName}>{item.name}</div>
          {section === "resources" && (
            <>
            <div className={styles.itemCount}>Кол-во: {item.item_count}</div>
            <button
              className={styles.useButton}
              onClick={() => handleUseItem(section, key, item)}
            >
              Использовать
            </button>
          </>
        )}
      </div>
    ));

  // 6) JSX
  return (
    <div className={styles.inventoryContainer}>
      <h2 className={styles.title}>Инвентарь</h2>
      <div className={styles.sections}>
        <div className={styles.section}>
          <h3 className={styles.sectionTitle}>Ресурсы</h3>
          <div className={styles.itemsContainer}>
            {renderItems("resources", resources)}
          </div>
        </div>
        <div className={styles.section}>
          <h3 className={styles.sectionTitle}>Артефакты</h3>
          <div className={styles.itemsContainer}>
            {renderItems("artifacts", artifacts)}
          </div>
        </div>
      </div>
    </div>
  );
};

export default Inventory;
