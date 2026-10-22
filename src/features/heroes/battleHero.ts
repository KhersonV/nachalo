type BattleHero = { id: string; owned: boolean; enabled: boolean; locked: boolean; active: boolean };
type BattleHeroState<T extends BattleHero> = { activeHeroClassId: string; heroes: T[] };

export function availableBattleHeroes<T extends BattleHero>(heroes: T[]): T[] {
    return heroes.filter(hero => hero.owned && hero.enabled && !hero.locked);
}

export function confirmedHeroActivation<T extends BattleHero>(state: BattleHeroState<T>, heroId: string): boolean {
    return state.activeHeroClassId === heroId && state.heroes.some(hero => hero.id === heroId && hero.active && hero.owned && hero.enabled && !hero.locked);
}
