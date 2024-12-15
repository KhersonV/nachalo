//inventory.tsx

"use client";

import React, { useState } from "react";
import "../styles/inventory.css";

// Тип для элемента инвентаря
type InventoryItem = {
  count: number; // Количество предметов
  description: string; // Описание предмета
};

// Тип для свойств компонента инвентаря
type InventoryProps = {
  items: Record<string, InventoryItem>; // Список предметов инвентаря
  onUseItem: (type: string) => void; // Функция для обработки использования предмета
};

// Компонент для отображения и управления инвентарем
export default function Inventory({ items, onUseItem }: InventoryProps) {
  // Локальное состояние для фильтрации предметов по названию
  const [filter, setFilter] = useState("");

  // Обработчик изменения значения фильтра
  const handleFilterChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setFilter(e.target.value.toLowerCase()); // Преобразуем текст в нижний регистр для поиска
  };

  // Фильтруем предметы по фильтру и исключаем "бочку" (barrel)
  const filteredItems = Object.entries(items)
    .filter(([key]) => key.toLowerCase() !== "barrbel") // Исключаем элемент "бочка"
    .filter(([key]) => key.toLowerCase().includes(filter)); // Применяем фильтр для поиска

  // Функция для получения пути к изображению ресурса
  const getResourceImagePath = (resourceType: string): string => {
    return `/main_resources/${resourceType}.webp`; // Формируем путь к изображению ресурса
  };

  return (
    <div className="inventory">
      {/* Поле для ввода текста фильтра */}
      <input
        className="inventory-search"
        type="text"
        placeholder="Поиск предметов..." // Подсказка для пользователя
        value={filter} // Текущее значение фильтра
        onChange={handleFilterChange} // Обработчик изменения значения фильтра
      />

      {/* Сетка для отображения предметов инвентаря */}
      <div className="inventory-grid">
        {/* Отображаем только отфильтрованные предметы */}
        {filteredItems.map(([key, { count, description }]) => (
          <div
            className="inventory-item"
            key={key} // Уникальный ключ для каждого элемента
            onClick={() => onUseItem(key)} // Обработчик клика для использования предмета
          >
            {/* Изображение ресурса */}
            <img
              src={getResourceImagePath(key)} // Формируем путь к изображению
              alt={key} // Альтернативный текст для изображения
              className="inventory-image"
            />
            {/* Название ресурса */}
            <p>{key}</p>
            {/* Количество ресурса */}
            <p>Количество: {count}</p>
            {/* Описание ресурса */}
            <p className="inventory-description">{description}</p>
            {/* Подсказка для пользователя */}
            <p className="inventory-action-hint">Нажмите для использования</p>
          </div>
        ))}
      </div>
    </div>
  );
}
