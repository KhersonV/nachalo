"use client";

import React from "react";
import { PlayerState } from "./GameContext";
import "../styles/player.css";

type PlayerProps = {
  player: PlayerState;
  isActive: boolean;
};

export default function Player({ player, isActive }: PlayerProps) {
  // Первый игрок - красный, второй - синий
  const colorClass = player.id === 0 ? "red-player" : "blue-player";

  return (
    <div className={`player-info ${colorClass} ${isActive ? "active" : ""}`}>
      {player.name} (HP:{player.health}, Energy:{player.energy})
    </div>
  );
}
