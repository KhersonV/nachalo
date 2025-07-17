
//=====================================
// src/components/TurnIndicator.tsx
//=====================================

"use client";

import React from "react";
import { useSelector } from "react-redux";
import type { RootState } from "../store";

const TurnIndicator = () => {
  const turnNumber = useSelector((state: RootState) => state.game.turnNumber);

  return (
    <div style={{ padding: "0.5rem", backgroundColor: "#222", color: "#fff", borderRadius: "4px", width: 100 }}>
      <p>Turn: {turnNumber}</p>
    </div>
  );
};

export default TurnIndicator;
