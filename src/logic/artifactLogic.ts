// src/logic/artifactLogic.ts
import { useGameContext } from "../components/GameContext";

export function useArtifactLogic() {
  const { state, setState } = useGameContext();

  /**
   * Обрабатывает взятие артефакта игроком.
   */
  const pickArtifact = (playerId: number) => {
    setState((prev) => {
      if (!prev.grid) return prev;

      const player = prev.players.find((p) => p.id === playerId);
      if (!player) return prev;

      const cell = prev.grid.find(
        (c) => c.x === player.position.x && c.y === player.position.y
      );

      console.log(`Игрок ${player.name} взял артефакт!`);

      return { ...prev, artifactOwner: playerId };
    });
  };

  /**
   * Обрабатывает потерю артефакта игроком.
   */
  const loseArtifact = (playerId: number) => {
    setState((prev) => {
      if (prev.artifactOwner !== playerId) return prev;

      console.log(`Игрок с ID ${playerId} потерял артефакт.`);

      return { ...prev, artifactOwner: null };
    });
  };

  /**
   * Уведомляет владельца артефакта.
   */
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
