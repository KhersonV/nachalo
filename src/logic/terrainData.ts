export const terrainData: Record<string, {image:string; defenseModifier:number}> = {
    ground: { image: "./main_tails/ground.webp", defenseModifier:0 },
    forest: { image: './main_tails/forest.webp', defenseModifier:1 },
    mountain: { image: './main_tails/mountain.webp', defenseModifier:2 },
    ice: { image: './main_tails/ice.webp', defenseModifier:-1 },
    water: { image: './main_tails/river.webp', defenseModifier:-2 },
    grass: {image: './main_tails/grass.webp', defenseModifier:0 },
    // Добавьте свои пути к картинкам террейна
  };
  