
//==================================
// src/components/GameController.tsx
//==================================

"use client";

import React, { useEffect, useState } from "react";
import { useGame } from "../contexts/GameContextt";
import { useAuth } from "../contexts/AuthContext";
import MapWithCamera from "./MapWithCamera";
import Controls from "./Controls";
import type { PlayerState, Cell } from "../types/GameTypes";
import EndTurnButton from "./EndTurnButton";
import TurnIndicator from "./TurnIndicator";
import Inventory from "./Inventory";
import styles from "../styles/GameController.module.css";
import PlayerHUD from "./PlayerHUD";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8001";

interface CollectResponse {
  updatedCell: {
    cell_id: number;
    x: number;
    y: number;
    tileCode: string;
    resource: any | null;
    barbel: any | null;
    monster: any | null;
    isPortal: boolean;
    isPlayer: boolean;
  };
}


export default function GameController() {
  const { state, dispatch } = useGame();

  const { user } = useAuth();
  const { state: { instanceId } } = useGame(); // предполагаем, что в контексте есть instanceId


  const myPlayerId = user?.id;
  const myPlayer: PlayerState | undefined = state.players.find(
    (p) => p.user_id === myPlayerId
  );
  const isMyTurn = myPlayerId === state.active_user;

  const [showInventory, setShowInventory] = useState(false);

  useEffect(() => {
    // console.log(
    //   "[GameController] myPlayerId:",
    //   myPlayerId,
    //   "active_user:",
    //   state.active_user
    // );
  }, [myPlayerId, state.active_user]);

  // Универсальный обработчик: шаг или удар

  async function handleMoveOrAttack(direction: "up" | "down" | "left" | "right") {
    if (!myPlayer) {
      console.warn("Мой игрок не найден");
      return;
    }
    if (!isMyTurn) {
      console.warn("Сейчас не ваш ход");
      return;
    }

    // 1) Вычисляем соседнюю клетку
    const deltas = {
      up: { x: 0, y: -1 },
      down: { x: 0, y: 1 },
      left: { x: -1, y: 0 },
      right: { x: 1, y: 0 },
    } as const;
    const { x: dx, y: dy } = deltas[direction];
    const targetX = myPlayer.position.x + dx;
    const targetY = myPlayer.position.y + dy;

    // 2) Находим эту клетку в grid
    const targetCell = state.grid.find(c => c.x === targetX && c.y === targetY);

    if (!targetCell) {

      return;
    }

    if (targetCell.monster) {
    }

    // 3) Далее — единый запрос

    const updatedPlayer = await movePlayer(targetX, targetY);

    if (!updatedPlayer) {
      console.error("[handleMoveOrAttack] Не удалось выполнить действие на сервере");
      return;
    }

    dispatch({
      type: "UPDATE_PLAYER",
      payload: { instanceId, player: updatedPlayer },
    });
  }

  // 5) По необходимости можно затем подтянуть всю матрицу:
  // const freshMatch = await fetchMatch(instanceId);
  // dispatch({ type: "SET_MATCH_DATA", payload: freshMatch });


  // Запрос на ход или атаку
  async function movePlayer(newPosX: number, newPosY: number) {
    try {
      const storedUser = localStorage.getItem("user");
      const token = storedUser ? JSON.parse(storedUser).token : "";
      if (!token) {
        console.error("[movePlayer] Токен не найден");
        return null;
      }

      const url = `http://localhost:8001/game/${instanceId}/player/${myPlayerId}/move`;
      const body = { new_pos_x: newPosX, new_pos_y: newPosY };

      const response = await fetch(url, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(body),
      });

      const text = await response.text();

      if (!response.ok) {
        console.error("[movePlayer] Ошибка перемещения:", text);
        return null;
      }

      const updatedPlayer = JSON.parse(text);
      return updatedPlayer as PlayerState;
    } catch (error) {
      console.error("[movePlayer] Ошибка запроса:", error);
      return null;
    }
  }






  // Функция для открытия бочки
  async function openBarrel(
    cellX: number,
    cellY: number
  ): Promise<
    | { updatedCell: Cell; updatedPlayer: PlayerState; matchEnded: true }
    | { updatedCell: Cell; updatedPlayer: PlayerState }
    | null
  > {
    console.log("Открываем бочку в клетке:", cellX, cellY);
    try {
      // 1. Получаем токен
      const stored = localStorage.getItem("user");
      const token = stored ? JSON.parse(stored).token : "";
      if (!token) {
        console.error("Токен не найден");
        return null;
      }

      // 2. Посылаем запрос
      const res = await fetch("http://localhost:8001/game/openBarrel", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          instance_id: instanceId,
          user_id: myPlayerId,
          cell_x: cellX,
          cell_y: cellY,
        }),
      });

      if (!res.ok) {
        console.error("Ошибка открытия бочки:", await res.text());
        return null;
      }

      // 3. Парсим ответ
      const data = (await res.json()) as {
        updatedCell?: any;
        updatedPlayer?: any;
        matchEnded?: boolean;
      };
      console.log("Бочка открыта, данные:", data);

      if (!data.updatedCell || !data.updatedPlayer) {
        console.warn("openBarrel: сервер вернул некорректные данные", data);
        return null;
      }

      // 4. Мапим клетку
      const mappedCell: Cell = {
        cell_id: data.updatedCell.cell_id,
        x: data.updatedCell.x,
        y: data.updatedCell.y,
        tileCode: data.updatedCell.tileCode,
        resource: data.updatedCell.resource ?? null,
        barbel: data.updatedCell.barbel ?? null,
        monster: data.updatedCell.monster ?? null,
        isPortal: data.updatedCell.isPortal,
        isPlayer: data.updatedCell.isPlayer ?? false,
      };

      // 5. Мапим игрока — здесь position не меняется, просто приводим к типу
      const mappedPlayer: PlayerState = data.updatedPlayer as PlayerState;

      // 6. Возвращаем результат
      if (data.matchEnded) {
        return { updatedCell: mappedCell, updatedPlayer: mappedPlayer, matchEnded: true };
      }
      return { updatedCell: mappedCell, updatedPlayer: mappedPlayer };
    } catch (e) {
      console.error("Ошибка запроса открытия бочки:", e);
      return null;
    }
  }









  // Функция сбора ресурсов
  async function collectResource(cellX: number, cellY: number) {
    console.log("Начало сбора ресурса для клетки:", cellX, cellY);

    // 1) Базовые проверки
    if (!instanceId) {
      console.error("collectResource: instanceId не задан");
      return;
    }
    if (!user) {
      console.error("collectResource: пользователь не авторизован");
      return;
    }

    const token = user.token;
    const myPlayerId = user.id;

    if (!token) {
      console.error("collectResource: токен пользователя отсутствует");
      return;
    }

    try {
      // 2) POST на сбор ресурса
      const response = await fetch("http://localhost:8001/game/collectResource", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${token}`,
        },
        body: JSON.stringify({
          instance_id: instanceId,
          user_id: myPlayerId,
          cell_x: cellX,
          cell_y: cellY,
        }),
      });

      if (!response.ok) {
        const errorText = await response.text();
        console.error("Ошибка сбора ресурса:", errorText);
        return;
      }

      const { updatedCell } = await response.json();
      console.log("Ресурс успешно собран. Полученные данные клетки:", updatedCell);

      // 3) Обновляем локальное состояние клетки (пример)
      const newCell = {
        cell_id: updatedCell.cell_id,
        x: updatedCell.x,
        y: updatedCell.y,
        tileCode: updatedCell.tileCode,
        resource: updatedCell.resource || null,
        barbel: updatedCell.barbel || null,
        monster: updatedCell.monster || null,
        isPortal: updatedCell.isPortal,
        isPlayer: updatedCell.isPlayer ?? false,
      };
      // dispatch({ type: "UPDATE_CELL", payload: newCell });  // если используете редьюсер

      // 4) GET для получения обновлённых данных игрока
      const playerRes = await fetch(
        `http://localhost:8001/game/matches/${instanceId}/players/${myPlayerId}`,
        {
          headers: {
            "Content-Type": "application/json",
            "Authorization": `Bearer ${token}`,
          },
        }
      );

      if (!playerRes.ok) {
        const err = await playerRes.text();
        console.error("Ошибка запроса обновлённого игрока:", err);
        return;
      }


      const updatedPlayer = await playerRes.json();
      console.log("Получили обновлённого игрока:", updatedPlayer);

      // 5) диспатчим обновлённого игрока
      dispatch({
        type: "UPDATE_PLAYER",
        payload: {
          instanceId: state.instanceId,
          player: updatedPlayer,
        },
      });
    } catch (e) {
      console.error("collectResource: запрос завершился с ошибкой", e);
    }
  }






  // Функция перемещения: вычисляем новые координаты и отправляем запрос на сервер.
  const handleMove = async (direction: "up" | "down" | "left" | "right") => {
    if (!myPlayer) {
      console.warn("Мой игрок не найден");
      return;
    }
    if (!isMyTurn) {
      console.warn("Сейчас не ваш ход");
      return;
    }
    let newPos = { ...myPlayer.position };
    if (direction === "up") newPos.y -= 1;
    else if (direction === "down") newPos.y += 1;
    else if (direction === "left") newPos.x -= 1;
    else if (direction === "right") newPos.x += 1;

    // Отправляем запрос перемещения на сервер (без проверок на клиенте)
    const updatedPlayer = await movePlayer(newPos.x, newPos.y);
    if (updatedPlayer) {
      dispatch({
        type: "UPDATE_PLAYER",
        payload: { instanceId, player: updatedPlayer },
      });
    } else {
      console.log("Перемещение не выполнено из-за ошибки сервера.");
    }
  };

  // Функция действий на клетке: сервер принимает решение, что делать.

  const [isCollecting, setIsCollecting] = useState(false);







  const handleAction = () => {
    if (!isMyTurn) {
      console.warn("Сейчас не ваш ход");
      return;
    }
    if (isCollecting) return; // предотвращаем повторные вызовы
    setIsCollecting(true);

    const activePlayer = state.players.find((p) => p.user_id === state.active_user);
    if (!activePlayer) {
      setIsCollecting(false);
      return;
    }
    const currentCell = state.grid.find(
      (cell) =>
        cell.x === activePlayer.position.x &&
        cell.y === activePlayer.position.y
    );
    if (!currentCell) {
      console.log("Клетка не найдена");
      setIsCollecting(false);
      return;
    }

    if (currentCell.monster) {
      console.log("Начинаем бой с монстром", currentCell.monster);
      setIsCollecting(false);
      return;
    }
    if (currentCell.resource) {
      console.log("Собираем ресурс", currentCell.resource);
      collectResource(currentCell.x, currentCell.y)
        .finally(() => setIsCollecting(false));
      return;
    }
    if (currentCell.barbel) {
      openBarrel(currentCell.x, currentCell.y)
        .finally(() => setIsCollecting(false));
      return;
    }

    if (currentCell.isPortal) {
      console.log("Пытаемся выйти через портал");
      setIsCollecting(false);
      return;
    }
    console.log("На данной клетке нет интерактивных объектов", currentCell);
    setIsCollecting(false);
  };

  const handleTurnEnded = (data: { active_user: number; turnNumber: number; energy: number }) => {
    dispatch({
      type: "SET_ACTIVE_USER",
      payload: { instanceId, active_user: data.active_user, turnNumber: data.turnNumber, energy: data.energy, },
    });
  };

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", " ", "i", "I", "ш", "Ш"].includes(e.key)) {
        e.preventDefault();
      }
      if (e.key === "ArrowUp") {
        handleMoveOrAttack("up");
      } else if (e.key === "ArrowDown") {
        handleMoveOrAttack("down");
      } else if (e.key === "ArrowLeft") {
        handleMoveOrAttack("left");
      } else if (e.key === "ArrowRight") {
        handleMoveOrAttack("right");
      } else if (e.key === " ") {
        handleAction();
      } else if (e.key.toLowerCase() === "i" || e.key.toLowerCase() === "ш") {
        setShowInventory((prev) => !prev);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [state, myPlayer, isMyTurn]);

  return (
    <div className={styles.container}>
      {myPlayer && (
        <PlayerHUD
          health={myPlayer.health}
          maxHealth={myPlayer.maxHealth}
          energy={myPlayer.energy}
          maxEnergy={myPlayer.maxEnergy}
        />
      )}
      <div className={styles.mapContainer}>
        {myPlayer ? (
          <MapWithCamera
            tileSize={80}
            viewportWidth={800}
            viewportHeight={600}
            myPlayer={myPlayer}
          />
        ) : (
          <p>Загрузка карты...</p>
        )}
      </div>
      <div className={styles.controlsContainer}>
        {isMyTurn ? (
          <>
            <Controls onMove={handleMove} onAction={handleAction} />
            <EndTurnButton
              playerId={myPlayerId!}
              instanceId={instanceId}
              onTurnEnded={handleTurnEnded}
            />
            <TurnIndicator />
          </>
        ) : (
          <div className={styles.waitingOverlay}>
            <p>Ожидание хода...</p>
          </div>
        )}
      </div>
      {showInventory && <Inventory />}
    </div>
  );
}
