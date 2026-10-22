import type { BaseState, BuildingState, HeroesState, HeroState } from "../../types/LobbyTypes";

export type BuildingKey = "forge" | "library" | "tavern";
export type ResourceKey = "wood" | "stone" | "iron";

export const BUILDING_KEYS: readonly BuildingKey[] = ["forge", "library", "tavern"];
export const RESOURCE_KEYS: readonly ResourceKey[] = ["wood", "stone", "iron"];

export function selectedBuilding(state: BaseState, key: BuildingKey): BuildingState {
    return state[key];
}

export function buildingStatus(building: BuildingState): "built" | "ready" | "missing" | "unavailable" {
    if (building.built) return "built";
    if (building.canBuild) return "ready";
    if (RESOURCE_KEYS.some(key => resourceMissing(building, key) > 0)) return "missing";
    return "unavailable";
}

export function resourceMissing(building: BuildingState, key: ResourceKey): number {
    return Math.max(0, building.costs[key] - building.resources[key]);
}

export function buildActionAllowed(building: BuildingState, busy: boolean): boolean {
    return !busy && !building.built && building.canBuild;
}

export function heroAction(hero: HeroState): "active" | "activate" | "hire" | "locked" {
    if (hero.active) return "active";
    if (hero.owned) return "activate";
    return hero.canHire ? "hire" : "locked";
}

export function confirmedHeroAction(current: HeroesState, updated: HeroesState, id: string, action: "hire" | "activate"): boolean {
    const before = current.heroes.find(hero => hero.id === id);
    const after = updated.heroes.find(hero => hero.id === id);
    if (!before || !after) return false;
    return action === "hire" ? after.owned : after.owned && after.active && updated.activeHeroClassId === id;
}
