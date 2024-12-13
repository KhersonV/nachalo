//terrainData.ts

export const terrainData: Record<
  string,
  { image: string; defenseModifier: number; transition_to?: Record<string, string> }
> = {
  ground: {
    image: "./main_tails/ground.webp",
    defenseModifier: 0,
    transition_to: {
      grass: "./mixed_tails/ground_to/ground_grass_transition.webp",
      river: "./mixed_tails/ground_to/ground_river_transition.webp",
      forest: "./mixed_tails/ground_to/ground_forest_transition.webp",
      mountain: "./mixed_tails/ground_to/ground_mountain_transition.webp",
      ice: "./mixed_tails/ground_to/ground_ice_transition.webp",
    },
  },
  forest: {
    image: "./main_tails/forest.webp",
    defenseModifier: 1,
    transition_to: {
      grass: "./mixed_tails/forest_to/forest_grass_transition.webp",
      ground: "./mixed_tails/forest_to/forest_ground_transition.webp",
      mountain: "./mixed_tails/forest_to/forest_mountain_transition.webp",
      river: "./mixed_tails/forest_to/forest_river_transition.webp",
      ice: "./mixed_tails/forest_to/forest_ice_transition.webp",
    },
  },
  mountain: {
    image: "./main_tails/mountain.webp",
    defenseModifier: 2,
    transition_to: {
      grass: "./mixed_tails/mountain_to/mountain_grass_transition.webp",
      forest: "./mixed_tails/mountain_to/mountain_forest_transition.webp",
      ground: "./mixed_tails/mountain_to/mountain_ground_transition.webp",
      river: "./mixed_tails/mountain_to/mountain_river_transition.webp",
      ice: "./mixed_tails/mountain_to/mountain_ice_transition.webp",
    },
  },
  ice: {
    image: "./main_tails/ice.webp",
    defenseModifier: -1,
    transition_to: {
      river: "./mixed_tails/ice_to/ice_river_transition.webp",
      grass: "./mixed_tails/ice_to/ice_grass_transition.webp",
      mountain: "./mixed_tails/ice_to/ice_mountain_transition.webp",
      ground: "./mixed_tails/ice_to/ice_ground_transition.webp",
      forest: "./mixed_tails/ice_to/ice_forest_transition.webp",
    },
  },
  river: {
    image: "./main_tails/river.webp",
    defenseModifier: -2,
    transition_to: {
      grass: "./mixed_tails/river_to/river_grass_transition.webp",
      ground: "./mixed_tails/river_to/river_ground_transition.webp",
      forest: "./mixed_tails/river_to/river_forest_transition.webp",
      ice: "./mixed_tails/river_to/river_ice_transition.webp",
      mountain: "./mixed_tails/river_to/river_mountain_transition.webp",
    },
  },
  grass: {
    image: "./main_tails/grass.webp",
    defenseModifier: 0,
    transition_to: {
      river: "./mixed_tails/grass_to/grass_river_transition.webp",
      ice: "./mixed_tails/grass_to/grass_ice_transition.webp",
      ground: "./mixed_tails/grass_to/grass_ground_transition.webp",
      forest: "./mixed_tails/grass_to/grass_forest_transition.webp",
      mountain: "./mixed_tails/grass_to/grass_mountain_transition.webp",
    },
  },
};
