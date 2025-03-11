
//=====================================
// src/components/TurnIndicator.tsx
//=====================================

"use client";

import React from "react";
import { useGame } from "../contexts/GameContextt";

const TurnIndicator = () => {
  const { state } = useGame();

  return (
    <div style={{ padding: "0.5rem", backgroundColor: "#222", color: "#fff", borderRadius: "4px", width: 100 }}>
      <p>Turn: {state.turnNumber}</p>
    </div>
  );
};

export default TurnIndicator;
