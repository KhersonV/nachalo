// src/logic/monsters.test.ts

import { aggressiveMonstersAttack } from "./monsters";
import { GameState, PlayerState, Cell, MonsterState } from "./types";
import { Action } from "./actions";

describe("aggressiveMonstersAttack", () => {
  it("should attack each monster once per turnCycle", () => {
    const initialState: GameState = {
      mode: "1v1",
      players: [
        {
          id: 0,
          name: "Player1",
          position: { x: 5, y: 5 },
          energy: 100,
          maxEnergy: 100,
          level: 1,
          expirience: 0,
          max_expirience: 500,
          visionRange: 5,
          health: 100,
          maxHealth: 100,
          attack: 10,
          defense: 5,
          image: "player-1.webp",
          inventory: {},
          abilities: {
            canMove: true,
            canAttack: true,
            canCollectResources: true,
            canUseItems: true,
            canInteractWithObjects: true,
            canPassTurn: true,
            canPickArtifact: true,
            canLoseArtifact: true,
          },
        },
      ],
      grid: [
        {
          id: 0,
          x: 5,
          y: 5,
          terrain: "ground",
          resource: null,
          isPortal: false,
          monster: {
            id: 1,
            name: "Orc",
            type: "aggressive",
            hp: 100,
            maxHp: 100,
            attack: 20,
            defense: 5,
            vision: 5,
            image: {
              ground: "/orc-ground.webp",
              forest: "/orc-forest.webp",
            },
            lastTurnAttacked: undefined,
          },
        },
      ],
      mapWidth: 10,
      mapHeight: 10,
      artifactOwner: null,
      portalPosition: null,
      instanceId: "test-instance",
      currentPlayerIndex: 0,
      turnCycle: 2,
      inventoryOpen: false,
        monstersHaveAttacked: false,
    };

    const mockDispatch = jest.fn();

    aggressiveMonstersAttack(initialState, mockDispatch);

    // Проверяем, что dispatch был вызван один раз для монстра
    expect(mockDispatch).toHaveBeenCalledTimes(2); // ATTACK и UPDATE_MONSTER_ATTACK_TURN

    // Проверяем, что ATTACK был вызван с правильными параметрами
    expect(mockDispatch).toHaveBeenNthCalledWith(1, {
      type: "ATTACK",
      payload: {
        attackerId: 1,
        targetId: 0,
        damage: 15, // 20 (attack) - 5 (defense)
        targetType: "player",
      },
    });

    // Проверяем, что UPDATE_MONSTER_ATTACK_TURN был вызван с правильными параметрами
    expect(mockDispatch).toHaveBeenNthCalledWith(2, {
      type: "UPDATE_MONSTER_ATTACK_TURN",
      payload: {
        monsterId: 1,
        turnCycle: 2,
      },
    });
  });
});
