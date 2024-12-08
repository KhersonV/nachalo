// Player.tsx
"use client";

import { useEffect, useCallback } from "react";

type Position = {
  x: number;
  y: number;
};

export function usePlayerMovement(
  gridSize: number,
  setPlayerPosition: React.Dispatch<React.SetStateAction<Position>>
) {
  const movePlayer = useCallback(
    (dx: number, dy: number) => {
      setPlayerPosition((prev) => ({
        x: Math.max(0, Math.min(gridSize - 1, prev.x + dx)),
        y: Math.max(0, Math.min(gridSize - 1, prev.y + dy)),
      }));
    },
    [gridSize, setPlayerPosition]
  );

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      switch (e.key) {
        case "ArrowUp":
          movePlayer(0, -1);
          break;
        case "ArrowDown":
          movePlayer(0, 1);
          break;
        case "ArrowLeft":
          movePlayer(-1, 0);
          break;
        case "ArrowRight":
          movePlayer(1, 0);
          break;
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [movePlayer]);

  return movePlayer;
}
