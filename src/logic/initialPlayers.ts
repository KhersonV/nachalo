import { PlayerState, PlayerAbilities} from "./types"; // Убедитесь, что путь корректен

const defaultAbilities: PlayerAbilities = {
  canMove: true,
  canAttack: true,
  canCollectResources: true,
  canUseItems: true,
  canInteractWithObjects: true,
  canPassTurn: true,
  canPickArtifact: true,
  canLoseArtifact: true,
};

const initialPlayers: PlayerState[] = [
  {
    id: 0,
    name: "Player1",
    position: { x: 0, y: 0 },
    energy: 100,
    maxEnergy: 100,
    level: 1,
    experience: 0,
    maxExperience: 500, // Исправлено с max_experience на maxExperience
    visionRange: 5,
    health: 100,
    maxHealth: 100,
    attack: 10,
    defense: 5,
    speed: 3,           
    maneuverability: 2, 
    vision: 5, // Исправлено с visionRange: 5, на visionRange: 5,
    image: "player-1.webp", // Исправлено с объекта на строку
    inventory: {},
    abilities: { ...defaultAbilities },
  },
  {
    id: 1,
    name: "Player2",
    position: { x: 19, y: 19 },
    energy: 100,
    maxEnergy: 100,
    level: 1,
    experience: 0,
    maxExperience: 500, // Исправлено с max_experience на maxExperience
    visionRange: 3,
    vision: 5,
    health: 100,
    maxHealth: 100,
    attack: 10,
    defense: 5,
    speed: 3,            
    maneuverability: 2, 
    image: "player-2.webp",
    inventory: {},
    abilities: { ...defaultAbilities },
  },
];

export default initialPlayers;
