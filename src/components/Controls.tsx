// src/components/Controls.tsx
"use client";

import React from "react";

type ControlsProps = {
  onMove: (direction: "up" | "down" | "left" | "right") => void;
  onAction: (action: string) => void;
};

export default function Controls({ onMove, onAction }: ControlsProps) {
  return (
    <div
      style={{
        position: "absolute",
        bottom: 20,
        left: "50%",
        transform: "translateX(-50%)",
        display: "flex",
        gap: "10px",
      }}
    >
      <button onClick={() => onMove("up")}>↑</button>
      <button onClick={() => onMove("down")}>↓</button>
      <button onClick={() => onMove("left")}>←</button>
      <button onClick={() => onMove("right")}>→</button>
      <button onClick={() => onAction("attack")}>Пробел</button>
    </div>
  );
}
