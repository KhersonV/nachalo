
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//************************************************* src/logic/initialPlayers.ts ***********************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************



import { PlayerState } from "./types";


// Допустим, у нас 5 игроков в team1, 5 в team2:
const team1Colors = ["green-player", "red-player", "yellow-player", "purple-player", "white-player"];
const team2Colors = [ "pink-player", "blue-player", "brown-player", "gray-player", "orange-player"];

export function assignTeamsAndColors(players: PlayerState[]) {
  const half = players.length / 2; // например, 1, если всего 2 игрока
  for (let i = 0; i < players.length; i++) {
    if (i < half) {
      // команда 1
      players[i].colorClass = team1Colors[i];
    } else {
      // команда 2
      // Если half=1, то для i=1 получится 1 - 1 = 0 → team2Colors[0]
      players[i].colorClass = team2Colors[i - half];
    }
  }
}

const initialPlayers: PlayerState[] = [
  {
    id: 0,
    name: "Player1",
    position: { x: 0, y: 0 },
    energy: 100,
    maxEnergy: 100,
    level: 1,
    experience: 0,
    maxExperience: 500,
    visionRange: 5,
    health: 100,
    maxHealth: 100,
    attack: 10,
    defense: 5,
    speed: 3,           
    maneuverability: 2, 
    vision: 5,
    image: "player-1.webp",
    colorClass: "",
    inventory: {
      resources: {},
      artifacts: {},
    },
  },
  {
    id: 1,
    name: "Player2",
    position: { x: 19, y: 19 },
    energy: 100,
    maxEnergy: 100,
    level: 1,
    experience: 0,
    maxExperience: 500,
    visionRange: 3,
    vision: 5,
    health: 100,
    maxHealth: 100,
    attack: 10,
    defense: 5,
    speed: 3,            
    maneuverability: 2, 
    image: "player-2.webp",
    colorClass: "",
    inventory: {
      resources: {},
      artifacts: {},
    },
  },
];

export default initialPlayers;
