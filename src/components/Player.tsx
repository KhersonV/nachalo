"use client";

import React from "react";
import { PlayerState } from "./GameContext";

type PlayerProps = {
  player: PlayerState;
  isActive: boolean;
};

export default function Player({ player, isActive }: PlayerProps) {
  // Пока просто выводим информацию или можно ничего не делать,
  // т.к. сам игрок отображается на карте (Map+Tile)
  // Этот компонент может быть использован для отображения отдельного UI игрока,
  // его статов, индикаторов.
  
  return (
    <div style={{ color: isActive ? "yellow" : "white" }}>
      Игрок {player.id}: HP={player.health} / Energy={player.energy}
    </div>
  );
}
