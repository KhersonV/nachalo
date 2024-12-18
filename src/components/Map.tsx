// Map.tsx


"use client";

import React from "react";
import Tile from "./Tile"; // Импортируем компонент Tile для отображения отдельных клеток карты
import { Cell } from "../logic/types"; // Импортируем тип Cell для описания клеток карты
import "../styles/map.css"; // Подключаем стили для карты

// Тип свойств, которые принимает компонент Map
type MapProps = {
  grid: Cell[]; // Сетка клеток карты
  playerPositions: { x: number; y: number }[]; // Позиции всех игроков
  visionRange: number; // Радиус видимости активного игрока
  mapWidth: number; // Ширина карты
  mapHeight: number; // Высота карты
  activePlayerIndex: number; // Индекс активного игрока
};

// Компонент для отображения карты
export default function Map({
  grid,
  playerPositions,
  visionRange,
  mapWidth,
  mapHeight,
  activePlayerIndex,
}: MapProps) {
  // Получаем позицию активного игрока
  const activePlayerPos = playerPositions[activePlayerIndex];
  const { x, y } = activePlayerPos;

  // Рассчитываем границы видимой области карты для активного игрока
  const startX = Math.max(x - visionRange, 0); // Левая граница видимости
  const endX = Math.min(x + visionRange, mapWidth - 1); // Правая граница видимости
  const startY = Math.max(y - visionRange, 0); // Верхняя граница видимости
  const endY = Math.min(y + visionRange, mapHeight - 1); // Нижняя граница видимости

  // Фильтруем клетки, которые находятся в пределах видимости активного игрока
  const visibleTiles = grid.filter(
    (cell) => cell.x >= startX && cell.x <= endX && cell.y >= startY && cell.y <= endY
  );

  // Определяем количество строк и столбцов в видимой области карты
  const rowsCount = endY - startY + 1; // Количество строк
  const colsCount = endX - startX + 1; // Количество столбцов

  // Размер одной клетки карты (в пикселях)
  const tileSize = 80;

  // Рендерим карту в виде сетки
  return (
    <div
      className="map" // Класс для стилизации карты
      style={{
        display: "grid", // Используем CSS Grid для расположения клеток
        gridTemplateColumns: `repeat(${colsCount}, ${tileSize}px)`, // Устанавливаем количество столбцов и размер клеток
        gridTemplateRows: `repeat(${rowsCount}, ${tileSize}px)`, // Устанавливаем количество строк и размер клеток
        marginLeft: "100px", // Отступ карты от левого края
      }}
    >
      {visibleTiles.map((cell) => {
        // Определяем, какие игроки находятся на текущей клетке
        const playersOnThisCell = playerPositions
          .map((pos, index) => (pos.x === cell.x && pos.y === cell.y ? index : -1)) // Сравниваем позиции игроков с текущей клеткой
          .filter((idx) => idx !== -1); // Исключаем индексы для игроков, которые не находятся на клетке

        // Рендерим клетку карты
        return (
          <Tile
            key={cell.id} // Уникальный ключ для клетки
            cell={cell} // Передаём данные о клетке
            playersOnTile={playersOnThisCell} // Передаём индексы игроков, находящихся на этой клетке
          />
        );
      })}
    </div>
  );
}
