import test from 'node:test';
import assert from 'node:assert/strict';
import { compareItemBonuses, equipBlockReason, filterEquipmentItems, getAvailableEquipment, getGearCount } from './equipmentView.ts';

const hero = { characterId: 7, heroClassId: 'guardian', name: 'Guardian', level: 3 };
const sword = { instanceId: 'sword', slot: 'main_hand', rarity: 'green', bonuses: { attack: 2, defense: -1 }, status: 'inventory', classRestriction: 'guardian' };
const axe = { instanceId: 'axe', slot: 'main_hand', rarity: 'purple', bonuses: { attack: 5, defense: -3 }, status: 'inventory', classRestriction: 'berserker' };
const shield = { instanceId: 'shield', slot: 'off_hand', rarity: 'blue', bonuses: { defense: 3 }, status: 'inventory' };
const highLevel = { instanceId: 'helm', slot: 'helmet', rarity: 'orange', bonuses: {}, status: 'inventory', levelRequirement: 5 };
const equipped = { ...sword, status: 'equipped', equippedCharacterId: 7 };

test('eligibility follows ownership status, class, level, and two-handed off-hand rule', () => {
    assert.equal(equipBlockReason(sword, hero, {}), null);
    assert.equal(equipBlockReason(axe, hero, {}), 'Different hero class');
    assert.equal(equipBlockReason(highLevel, hero, {}), 'Requires hero level 5');
    assert.equal(equipBlockReason(shield, hero, { main_hand: { handedness: 'two_hand' } }), 'Blocked by two-handed weapon');
    assert.equal(equipBlockReason(equipped, hero, {}), 'Already equipped');
});

test('filters use only server item fields and selected hero eligibility', () => {
    const items = [sword, axe, shield, highLevel, equipped];
    const base = { slot: 'all', rarity: 'all', heroClass: 'all', availableOnly: false };
    assert.deepEqual(filterEquipmentItems(items, { ...base, slot: 'main_hand' }, hero, {}).map(item => item.instanceId), ['sword', 'axe', 'sword']);
    assert.deepEqual(filterEquipmentItems(items, { ...base, heroClass: 'guardian' }, hero, {}).map(item => item.instanceId), ['sword', 'shield', 'helm', 'sword']);
    assert.deepEqual(filterEquipmentItems(items, { ...base, rarity: 'blue' }, hero, {}).map(item => item.instanceId), ['shield']);
    assert.deepEqual(filterEquipmentItems(items, { ...base, availableOnly: true }, hero, {}).map(item => item.instanceId), ['sword', 'shield']);
});

test('comparison uses only known item bonus fields, not predicted combat totals', () => {
    const rows = compareItemBonuses(sword, axe);
    assert.deepEqual(rows.map(({ key, currentValue, selectedValue, difference }) => ({ key, currentValue, selectedValue, difference })), [
        { key: 'attack', currentValue: 2, selectedValue: 5, difference: 3 },
        { key: 'defense', currentValue: -1, selectedValue: -3, difference: -2 },
    ]);
});

test('server inventory keeps only free instances, including a free copy of an equipped template', () => {
    const freeCopy = { ...sword, instanceId: 'sword-free' };
    const wornCopy = { ...sword, instanceId: 'sword-worn', status: 'equipped', equippedCharacterId: 7 };
    const inconsistent = { ...sword, instanceId: 'other-hero' };
    const unknown = { ...sword, instanceId: 'unknown', status: undefined };
    const serverList = [freeCopy, wornCopy, inconsistent, unknown];
    const worn = { '7': { main_hand: wornCopy }, '8': { main_hand: { ...inconsistent, status: 'equipped' } } };
    assert.deepEqual(getAvailableEquipment(serverList, worn, {}).map(item => item.instanceId), ['sword-free']);
    assert.deepEqual(getAvailableEquipment([freeCopy], worn, {}).map(item => item.instanceId), ['sword-free']);
    assert.deepEqual(getAvailableEquipment([freeCopy, wornCopy], {}, {}).map(item => item.instanceId), ['sword-free']);
    assert.deepEqual(getAvailableEquipment(undefined, worn, {}), []);
});

test('gear count uses server confirmed two-handed property and frees both slots on removal', () => {
    const bow = { ...sword, handedness: 'two_hand', status: 'equipped' };
    const base = { helmet: { ...highLevel, status: 'equipped' } };
    assert.deepEqual(getGearCount({ ...base, main_hand: bow }), { equippedItemsCount: 2, occupiedSlotsCount: 3 });
    assert.deepEqual(getGearCount(base), { equippedItemsCount: 1, occupiedSlotsCount: 1 });
    assert.deepEqual(getGearCount({ ...base, main_hand: { ...bow, handedness: 'none', isTwoHanded: true } }), { equippedItemsCount: 2, occupiedSlotsCount: 3 });
    assert.deepEqual(getGearCount({ ...base, main_hand: { ...bow, handedness: 'one_hand' } }), { equippedItemsCount: 2, occupiedSlotsCount: 2 });
});

test('fresh server snapshots control availability across equip, unequip, and hero selection', () => {
    const item = { ...sword, instanceId: 'copy-1' };
    const free = getAvailableEquipment([item], { '7': {}, '8': {} }, {});
    const afterEquip = getAvailableEquipment([], { '7': {}, '8': { main_hand: { ...item, status: 'equipped' } } }, {});
    const afterUnequip = getAvailableEquipment([item], { '7': {}, '8': {} }, {});
    assert.deepEqual(free.map(({ instanceId }) => instanceId), ['copy-1']);
    assert.deepEqual(afterEquip, []);
    assert.deepEqual(afterUnequip.map(({ instanceId }) => instanceId), ['copy-1']);
});
