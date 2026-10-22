import test from 'node:test';
import assert from 'node:assert/strict';
import { toHexHudView } from './hexHudView.ts';

const base = () => ({
  stateRevision: 4, instanceId: 'hex-1', activeUser: 7, turnNumber: 2,
  turnDeadline: '2026-01-01T00:01:00.000Z', turnSeconds: 60,
  player: { userId: 7, name: 'Hero', characterType: 'guardian', health: 30, maxHealth: 100, energy: 12, maxEnergy: 90, position: { q: 0, r: 0 }, inventory: { resource_1: { item_count: 2 }, scroll_3: { item_count: 1 } } },
  cells: [{ cellId: 1, position: { q: 0, r: 0 }, terrain: 'floor', visibility: 'visible' }, { cellId: 2, position: { q: 1, r: 0 }, terrain: 'rock', visibility: 'unexplored' }],
  actors: [{ id: 'monster-5', kind: 'monster', name: 'Visible', health: 10, position: { q: 0, r: 1 } }],
});

test('hero values, timer, inventory and turn come from current snapshot', () => {
  const view = toHexHudView(base(), null, null, 'monster', null, null, Date.parse('2026-01-01T00:00:29.000Z'));
  assert.equal(view.hero.health, 30);
  assert.equal(view.hero.energy, 12);
  assert.equal(view.inventoryCount, 3);
  assert.equal(view.remaining, 31);
  assert.equal(view.isMyTurn, true);
  assert.equal(toHexHudView({ ...base(), activeUser: 8 }, null, null, 'monster', null, null, 0).isMyTurn, false);
});

test('target and cell details disappear when absent from personal filtered snapshot', () => {
  const snapshot = base();
  const visible = toHexHudView(snapshot, { q: 0, r: 0 }, 5, 'monster', { targetId: 5, cost: 3 }, null, 0);
  assert.equal(visible.target.name, 'Visible');
  assert.equal(visible.preview.cost, 3);
  assert.equal(visible.cell.cellId, 1);
  const hidden = toHexHudView({ ...snapshot, actors: [] }, { q: 1, r: 0 }, 5, 'monster', { targetId: 5, cost: 3 }, null, 0);
  assert.equal(hidden.target, undefined);
  assert.equal(hidden.preview, undefined);
  assert.equal(hidden.cell, undefined);
});

test('result or death blocks commands before overall match finishes', () => {
  assert.equal(toHexHudView({ ...base(), result: { method: 'defeat' } }, null, null, 'monster', null, null, 0).isMyTurn, false);
  assert.equal(toHexHudView({ ...base(), player: { ...base().player, health: 0 } }, null, null, 'monster', null, null, 0).finished, true);
  assert.equal(toHexHudView(base(), null, null, 'monster', null, null, 0, true).isMyTurn, false);
});

test('old route and target cannot remain actionable after filtered reconnect snapshot', () => {
  const route = { path: [{ q: 0, r: 0 }], totalCost: 2, affordable: true };
  const prior = toHexHudView(base(), { q: 0, r: 0 }, 5, 'monster', { targetId: 5, cost: 2 }, route, 0);
  assert.equal(prior.route, route);
  const updated = toHexHudView({ ...base(), stateRevision: 5, cells: [], actors: [] }, { q: 0, r: 0 }, 5, 'monster', { targetId: 5, cost: 2 }, route, 0);
  assert.equal(updated.cell, undefined);
  assert.equal(updated.route, undefined);
  assert.equal(updated.target, undefined);
  assert.equal(updated.preview, undefined);
});
