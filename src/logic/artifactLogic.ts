import { useGameContext } from "../components/GameContext";

export function useArtifactLogic() {
  const { state, setState } = useGameContext();

  function pickArtifact(playerId:number) {
    setState(prev=>({...prev, artifactOwner: playerId}));
    notifyArtifactOwner(playerId);
  }

  function loseArtifact(playerId:number) {
    setState(prev=>({...prev, artifactOwner: null}));
  }

  function notifyArtifactOwner(playerId:number) {
    console.log(`Игрок ${playerId} теперь владеет артефактом`);
  }

  return { pickArtifact, loseArtifact, notifyArtifactOwner };
}
