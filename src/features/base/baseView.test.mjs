import test from 'node:test';
import assert from 'node:assert/strict';
import { BUILDING_KEYS, buildActionAllowed, buildingStatus, confirmedHeroAction, heroAction, resourceMissing, selectedBuilding } from './baseView.ts';

const forge = { level: 0, built: false, canBuild: false, costs: { wood: 10, stone: 8, iron: 2 }, resources: { wood: 7, stone: 8, iron: 0 }, unlockables: [] };

test('selected building and level come from server state', () => {
    const state = { forge, library: { ...forge, level: 1, built: true }, tavern: { ...forge, level: 0 } };
    assert.deepEqual(BUILDING_KEYS, ['forge', 'library', 'tavern']);
    assert.equal(selectedBuilding(state, 'library').level, 1);
    assert.equal(selectedBuilding(state, 'tavern').level, 0);
});

test('missing resources are display-only derivatives of server cost and balance', () => {
    assert.equal(resourceMissing(forge, 'wood'), 3);
    assert.equal(resourceMissing(forge, 'stone'), 0);
    assert.equal(resourceMissing(forge, 'iron'), 2);
});

test('server canBuild and built fields control construction', () => {
    assert.equal(buildingStatus(forge), 'missing');
    assert.equal(buildingStatus({ ...forge, resources: forge.costs }), 'unavailable');
    assert.equal(buildingStatus({ ...forge, canBuild: true }), 'ready');
    assert.equal(buildingStatus({ ...forge, level: 1, built: true }), 'built');
    assert.equal(buildActionAllowed({ ...forge, canBuild: true }, false), true);
    assert.equal(buildActionAllowed({ ...forge, canBuild: true }, true), false);
    assert.equal(buildActionAllowed({ ...forge, canBuild: true, built: true }, false), false);
});

test('hero cards show only server-confirmed hire and activation actions', () => {
    const locked = { id: 'mystic', owned: false, active: false, canHire: false };
    const hireable = { ...locked, canHire: true };
    const owned = { ...locked, owned: true };
    assert.equal(heroAction(locked), 'locked');
    assert.equal(heroAction(hireable), 'hire');
    assert.equal(heroAction(owned), 'activate');
    assert.equal(heroAction({ ...owned, active: true }), 'active');
});

test('server refresh confirms hero changes; recipes and research have no action', () => {
    const before = { activeHeroClassId: 'guardian', heroes: [{ id: 'mystic', owned: false, active: false }] };
    assert.equal(confirmedHeroAction(before, { ...before, heroes: [{ id: 'mystic', owned: true, active: false }] }, 'mystic', 'hire'), true);
    assert.equal(confirmedHeroAction(before, before, 'mystic', 'hire'), false);
    const active = { activeHeroClassId: 'mystic', heroes: [{ id: 'mystic', owned: true, active: true }] };
    assert.equal(confirmedHeroAction(before, active, 'mystic', 'activate'), true);
    assert.equal(confirmedHeroAction(before, { ...active, activeHeroClassId: 'guardian' }, 'mystic', 'activate'), false);
    assert.equal(confirmedHeroAction(before, { ...active, heroes: [{ id: 'mystic', owned: true, active: false }] }, 'mystic', 'activate'), false);
    assert.equal('craft' in forge, false);
    assert.equal('research' in forge, false);
});
