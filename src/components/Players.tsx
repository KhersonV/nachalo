//Players.tsx

"use client";

import React from "react";
import Player from "./Player";
import { PlayerState } from "../logic/types";

type PlayersProps = {
  players: PlayerState[];
  activePlayerId: number;
};

export default function Players({ players, activePlayerId }: PlayersProps) {
  return (
    <>
      {players.map((p) => (
        <Player key={p.id} player={p} isActive={p.id === activePlayerId} />
      ))}
    </>
  );
}
