"use client";

import { useState, useCallback } from "react";

export type PlayerState = {
  position: { x: number; y: number };
  energy: number;
  level: number;
  visionRange: number;
  health: number;
  attack: number;
  defense: number;
};

export function usePlayer(initialState: PlayerState) {
  const [state, setState] = useState(initialState);

  const move = useCallback(
    (dx: number, dy: number, mapWidth: number, mapHeight: number) => {
      setState((prev) => {
        const newX = Math.max(0, Math.min(mapWidth - 1, prev.position.x + dx));
        const newY = Math.max(0, Math.min(mapHeight - 1, prev.position.y + dy));
        return {
          ...prev,
          position: { x: newX, y: newY },
          energy: Math.max(0, prev.energy - 1),
        };
      });
    },
    []
  );

  const attack = useCallback(() => {
    setState((prev) => ({ ...prev, energy: Math.max(0, prev.energy - 2) }));
  }, []);

  const defend = useCallback(() => {
    setState((prev) => ({ ...prev, energy: Math.max(0, prev.energy - 1) }));
  }, []);

  return { state, move, attack, defend };
}
