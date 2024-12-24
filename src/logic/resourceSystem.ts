// src/logic/resourceSystem.ts

import { useGameContext } from "../components/GameContext";
import { createRandomMonster } from "./allData";
import { resources, getResource, artifacts } from "./allData"; // Импортируем artifacts
import { PlayerState, Cell, MonsterState } from "./types";

export function useResourceSystem() {
  const { state, dispatch } = useGameContext();

  /**
   * Открытие бочки:
   * 1. Удаляем бочку (REMOVE_RESOURCE).
   * 2. Случайный шанс (5% артефакт, 30% монстр, 65% обычный ресурс).
   * 3. Если артефакт — добавляем в инвентарь.
   * 4. Если монстр — добавляем монстра и запускаем бой (START_BATTLE).
   * 5. Если ресурс — добавляем в инвентарь.
   */
  const openBarrel = (playerId: number) => {
    if (!state.grid) {
      console.error("Grid is not initialized.");
      return;
    }

    const player: PlayerState | undefined = state.players.find((p) => p.id === playerId);
    if (!player) {
      console.error(`Игрок с ID ${playerId} не найден.`);
      return;
    }

    const targetX = player.position.x;
    const targetY = player.position.y;

    const targetCell: Cell | undefined = state.grid.find(
      (c) => c.x === targetX && c.y === targetY
    );

    if (!targetCell || !targetCell.resource || targetCell.resource.type !== "barrbel") {
      console.log("Бочка отсутствует на указанной клетке.");
      return;
    }

    console.log("Открываем бочку...");

    // Удаляем бочку сразу, чтобы она исчезла с карты
    dispatch({
      type: "REMOVE_RESOURCE",
      payload: { cellId: targetCell.id },
    });

    const random = Math.random();

    // ----- 5% шанс на артефакт -----
    if (random < 0.05) {
      // Случайно выбираем артефакт из массива artifacts
      const randomIndex = Math.floor(Math.random() * artifacts.length);
      const randomArtifact = artifacts[randomIndex];
      console.log(`Из бочки выпал артефакт: ${randomArtifact.name}`);

      // Добавляем артефакт в инвентарь
      dispatch({
        type: "ADD_ARTIFACT",
        payload: {
          playerId,
          artifactName: randomArtifact.name,
          description: randomArtifact.description,
          image: randomArtifact.image,
          bonus: randomArtifact.bonus,
        },
      });
    }
    // ----- 30% шанс на монстра (0.05 - 0.35) -----
    else if (random < 0.35) {
      const randomMonster: MonsterState = createRandomMonster();
      console.log(`Из бочки выпал монстр: ${randomMonster.name}`);

      // Добавляем монстра на клетку
      dispatch({
        type: "ADD_MONSTER",
        payload: { cellId: targetCell.id, monster: randomMonster },
      });

      // И сразу запускаем бой (монстр vs игрок)
      dispatch({
        type: "START_BATTLE",
        payload: {
          attacker: player,          // Игрок — атакующий
          defender: randomMonster,   // Монстр — защитник
          cellId: targetCell.id,
        },
      });
    }
    // ----- 65% шанс на ресурс (0.35 - 1.0) -----
    else {
      console.log("Из бочки выпал обычный ресурс!");
      // Собираем список ресурсов (исключая "barrbel")
      const possibleResources: string[] = Object.keys(resources).filter(
        (key) => key !== "barrbel"
      );
      // Рандомно выбираем ключ
      const randomResourceKey: string =
        possibleResources[Math.floor(Math.random() * possibleResources.length)];
      // Получаем данные о ресурсе
      const selectedResource = getResource(randomResourceKey);

      if (!selectedResource) {
        console.error(`Не удалось найти ресурс для ключа: ${randomResourceKey}`);
        return;
      }

      // Добавляем ресурс в инвентарь игрока
      dispatch({
        type: "ADD_RESOURCE",
        payload: {
          playerId,
          resourceType: randomResourceKey,
          description: selectedResource.description,
          image: selectedResource.image.ground,
        },
      });
    }
  };

  /**
   * Попытка выхода через портал.
   * Если у игрока есть артефакт (state.artifactOwner === playerId), считается, что он может выйти.
   */
  const tryExitThroughPortal = (playerId: number) => {
    if (!state.grid) {
      console.error("Grid is not initialized.");
      return;
    }

    const player: PlayerState | undefined = state.players.find((p) => p.id === playerId);
    if (!player) {
      console.error(`Игрок с ID ${playerId} не найден.`);
      return;
    }

    const cell: Cell | undefined = state.grid.find(
      (c) => c.x === player.position.x && c.y === player.position.y
    );
    if (!cell || !cell.isPortal) {
      console.log("Портал отсутствует на клетке.");
      return;
    }

    if (state.artifactOwner === playerId) {
      console.log(`Игрок ${player.name} успешно покидает портал с артефактом!`);
      // Здесь можно завершить инстанс:
      // dispatch({ type: "FINALIZE_INSTANCE", payload: { instanceId: state.instanceId, players: state.players } });
    } else {
      console.log("Игрок не может выйти через портал без артефакта.");
    }
  };

  /**
   * Собрать ресурс, если он есть на текущей клетке.
   */
  const collectResourceIfOnTile = (playerId: number) => {
    if (!state.grid) {
      console.error("Grid is not initialized.");
      return;
    }

    const player: PlayerState | undefined = state.players.find((p) => p.id === playerId);
    if (!player) {
      console.error(`Игрок с ID ${playerId} не найден.`);
      return;
    }

    const cell: Cell | undefined = state.grid.find(
      (c) => c.x === player.position.x && c.y === player.position.y
    );
    if (!cell || !cell.resource) {
      console.log("На клетке нет ресурса для сбора.");
      return;
    }

    dispatch({
      type: "COLLECT_RESOURCE",
      payload: { playerId, resourceType: cell.resource.type, cellId: cell.id },
    });
  };

  return {
    openBarrel,
    tryExitThroughPortal,
    collectResourceIfOnTile,
  };
}
