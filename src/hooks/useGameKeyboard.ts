
//==============================
// src/hooks/useGameKeyboard.ts
//==============================

import { useEffect } from "react";

type Direction = "up" | "down" | "left" | "right";

interface UseGameKeyboardProps {
  onMove: (direction: Direction) => void;
  onAction: () => void;
  onInventory: () => void;
}


export function useGameKeyboard({ onMove, onAction, onInventory }: UseGameKeyboardProps) {
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (
        [
          "ArrowUp",
          "ArrowDown",
          "ArrowLeft",
          "ArrowRight",
          " ",
          "i",
          "I",
          "ш",
          "Ш",
        ].includes(e.key)
      ) {
        e.preventDefault();
      }
      if (e.key === "ArrowUp") onMove("up");
      else if (e.key === "ArrowDown") onMove("down");
      else if (e.key === "ArrowLeft") onMove("left");
      else if (e.key === "ArrowRight") onMove("right");
      else if (e.key === " ") onAction();
      else if (e.key.toLowerCase() === "i" || e.key.toLowerCase() === "ш")
        onInventory();
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onMove, onAction, onInventory]);
}
