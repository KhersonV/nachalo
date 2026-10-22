import test from 'node:test';
import assert from 'node:assert/strict';
import { availableBattleHeroes, confirmedHeroActivation } from './battleHero.ts';

const heroes = [
    { id: 'guardian', owned: true, enabled: true, locked: false, active: true },
    { id: 'ranger', owned: true, enabled: true, locked: false, active: false },
    { id: 'mystic', owned: false, enabled: true, locked: true, active: false },
    { id: 'berserker', owned: true, enabled: false, locked: false, active: false },
];

test('hero picker exposes only owned, enabled, unlocked heroes', () => {
    assert.deepEqual(availableBattleHeroes(heroes).map(hero => hero.id), ['guardian', 'ranger']);
});

test('activation requires matching active class and hero in server response', () => {
    assert.equal(confirmedHeroActivation({ activeHeroClassId: 'ranger', heroes: heroes.map(hero => ({ ...hero, active: hero.id === 'ranger' })) }, 'ranger'), true);
    assert.equal(confirmedHeroActivation({ activeHeroClassId: 'guardian', heroes }, 'ranger'), false);
    assert.equal(confirmedHeroActivation({ activeHeroClassId: 'ranger', heroes }, 'ranger'), false);
    assert.equal(confirmedHeroActivation({ activeHeroClassId: 'mystic', heroes }, 'mystic'), false);
});
