
export type MonsterState = {
    name: string;
    type: 'aggressive'|'neutral';
    hp: number;
    maxHp: number;
    attack: number;
    defense: number;
    vision: number;
    image: string;
  };
  

export const allMonsters: MonsterState[] = [
    {
      name: "Goblin",
      type: "aggressive",
      hp: 20,
      maxHp: 20,
      attack: 25,
      defense: 1,
      vision: 3,
      image: "/goblin.webp"
    },
    {
      name: "Goblins",
      type: "neutral",
      hp: 40,
      maxHp: 40,
      attack: 50,
      defense: 2,
      vision: 1,
      image: "/goblins.webp"
    },
    {
      name: "Orc",
      type: "aggressive",
      hp: 50,
      maxHp: 50,
      attack: 30,
      defense: 5,
      vision: 2,
      image: "/orc.webp"
    },
    {
      name: "Troll",
      type: "neutral",
      hp: 80,
      maxHp: 80,
      attack: 40,
      defense: 10,
      vision: 1,
      image: "/troll.webp"
    }
  ];
  

  export function createRandomMonster(): MonsterState {
    const randomIndex = Math.floor(Math.random() * allMonsters.length);
    return allMonsters[randomIndex];
  }
  