import test from 'node:test';
import assert from 'node:assert/strict';
import { actionDisabled, clampShopQuantity, filterShopItems, getSellableEquipment, maxAffordableQuantity, purchaseBlockReason, purchaseTotal, selectionAfterSale } from './shopView.ts';

const food = { type: 'food', category: 'resource', price: 25, requiresForge: false };
const blueprint = { type: 'wall_blueprint', category: 'blueprint', price: 50, requiresForge: true };
const scroll = { type: 'scroll', category: 'scroll', price: 10, requiresForge: false };

test('quantity stays within server range and total follows the selected quantity', () => {
    assert.equal(clampShopQuantity(-5), 1);
    assert.equal(clampShopQuantity(120), 99);
    assert.equal(clampShopQuantity(2.9), 2);
    assert.equal(purchaseTotal(25, 5), 125);
});

test('Max respects gold and 99-item limit', () => {
    assert.equal(maxAffordableQuantity(125, 25), 5);
    assert.equal(maxAffordableQuantity(10000, 25), 99);
    assert.equal(maxAffordableQuantity(20, 25), 0);
    assert.equal(maxAffordableQuantity(null, 25), 0);
});

test('buy eligibility follows balance, Forge, and Library state', () => {
    assert.equal(purchaseBlockReason(food, 5, 100, true, true), 'Not enough gold');
    assert.equal(purchaseBlockReason(blueprint, 1, 100, false, true), 'Forge required');
    assert.equal(purchaseBlockReason(blueprint, 1, 100, null, true), 'Checking Forge…');
    assert.equal(purchaseBlockReason(scroll, 1, 100, true, false), 'Library required');
    assert.equal(purchaseBlockReason(scroll, 1, 100, true, true), null);
    assert.equal(actionDisabled(true, null), true);
    assert.equal(actionDisabled(false, 'Not enough gold'), true);
});

test('sell list uses free instance IDs and keeps an unequipped identical copy', () => {
    const copy = { instanceId: 'free', code: 'sword', status: 'inventory', sellPrice: 600 };
    const worn = { instanceId: 'worn', code: 'sword', status: 'equipped', sellPrice: 600 };
    const conflict = { instanceId: 'conflict', code: 'sword', status: 'inventory', sellPrice: 600 };
    const unknown = { instanceId: 'unknown', code: 'sword', sellPrice: 600 };
    assert.deepEqual(getSellableEquipment([copy, worn, conflict, unknown], { '2': { main_hand: conflict } }, {}).map(item => item.instanceId), ['free']);
    assert.deepEqual(getSellableEquipment(undefined, {}, {}), []);
    assert.equal(selectionAfterSale('free', 'free'), null);
    assert.equal(selectionAfterSale('another', 'free'), 'another');
});

test('coming soon preview never enters the purchasable catalog', () => {
    assert.deepEqual(filterShopItems([food, blueprint, scroll], 'preview'), []);
    assert.deepEqual(filterShopItems([food, blueprint, scroll], 'blueprint'), [blueprint]);
});
