// src/logic/artifactLogic.ts

import { useGameContext } from "../components/GameContext";
import { Action } from "./actions";

export function useArtifactLogic() {
  const { state, dispatch } = useGameContext();

  const pickArtifact = (playerId: number) => {
    console.log(`Игрок ${playerId} взял артефакт!`);
    dispatch({ type: 'PICK_ARTIFACT', payload: { playerId } });
  };

  const loseArtifact = (playerId: number) => {
    if (state.artifactOwner !== playerId) return;
    console.log(`Игрок с ID ${playerId} потерял артефакт.`);
    dispatch({ type: 'LOSE_ARTIFACT', payload: { playerId } });
  };

  const notifyArtifactOwner = () => {
    if (state.artifactOwner === null) {
      console.log("Артефакт не взят.");
      return;
    }

    const owner = state.players.find((p) => p.id === state.artifactOwner);
    if (owner) {
      console.log(`Владелец артефакта: ${owner.name}`);
    }
  };

  return {
    pickArtifact,
    loseArtifact,
    notifyArtifactOwner,
  };
}
