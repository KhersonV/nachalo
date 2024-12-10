export const terrainData: Record<string, {image:string; defenseModifier:number}> = {
    ground: { image: "ground.webp", defenseModifier:0 },
    forest: { image: 'forest.webp', defenseModifier:1 },
    mountain: { image: 'mountain.webp', defenseModifier:2 },
    ice: { image: 'ice.webp', defenseModifier:-1 },
    water: { image: 'river.webp', defenseModifier:-2 },
    // Добавьте свои пути к картинкам террейна
  };
  