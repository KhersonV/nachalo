import test from 'node:test';
import assert from 'node:assert/strict';
import { eligibleItems, nextLoadout, normalizedLoadout, remainingCount, SerializedLoadoutSaver } from './loadoutModel.ts';

const state = {
    inventory: {
        resource_1: { name: 'Food', item_count: 12 },
        scroll_2001: { name: 'Scroll', item_count: 2 },
        blueprint_wall: { name: 'Wall blueprint', item_count: 1 },
        equipment_sword: { name: 'Sword', item_count: 1 },
        resource_9: { name: 'Wood', item_count: 6 },
    },
    eligibleKeys: ['resource_1', 'scroll_2001', 'blueprint_wall'],
    loadout: { resource_1: 3, equipment_sword: 1 },
    balance: 0,
};

function deferred() {
    let resolve;
    let reject;
    const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
    return { promise, resolve, reject };
}

test('only server eligible keys appear and invalid saved keys are removed', () => {
    assert.deepEqual(eligibleItems(state).map(([key]) => key), ['blueprint_wall', 'resource_1', 'scroll_2001']);
    assert.deepEqual(normalizedLoadout(state), { resource_1: 3 });
});

test('available balance and quantity actions stay within owned count', () => {
    let loadout = { resource_1: 3 };
    loadout = nextLoadout(loadout, 'resource_1', 5, 12);
    assert.equal(loadout.resource_1, 8);
    assert.equal(remainingCount(12, loadout.resource_1), 4);
    loadout = nextLoadout(loadout, 'resource_1', -5, 12);
    assert.equal(loadout.resource_1, 3);
    loadout = nextLoadout(loadout, 'resource_1', -1, 12);
    assert.equal(loadout.resource_1, 2);
    loadout = nextLoadout(loadout, 'resource_1', 'max', 12);
    assert.equal(loadout.resource_1, 12);
    loadout = nextLoadout(loadout, 'resource_1', 5, 12);
    assert.equal(loadout.resource_1, 12);
    loadout = nextLoadout(loadout, 'resource_1', 'zero', 12);
    assert.deepEqual(loadout, {});
    assert.equal(remainingCount(12, 0), 12);
});

test('rapid edits persist one final full snapshot', async () => {
    const sent = [];
    const saver = new SerializedLoadoutSaver(async value => { sent.push(value); }, () => {}, 1000);
    saver.initialize({});
    saver.change({ resource_1: 5 });
    saver.change({ resource_1: 10 });
    saver.change({ resource_1: 9 });
    await saver.flush();
    assert.deepEqual(sent, [{ resource_1: 9 }]);
    saver.dispose();
});

test('stale response cannot mark newer edit saved and writes stay serial', async () => {
    const first = deferred();
    const second = deferred();
    const sent = [];
    const statuses = [];
    const saver = new SerializedLoadoutSaver(value => { sent.push(value); return sent.length === 1 ? first.promise : second.promise; }, status => statuses.push(status), 1000);
    saver.initialize({});
    saver.change({ resource_1: 5 });
    const flush = saver.flush();
    await Promise.resolve();
    saver.change({ resource_1: 10 });
    first.resolve();
    await Promise.resolve();
    await Promise.resolve();
    assert.equal(statuses.at(-1), 'saving');
    assert.deepEqual(sent, [{ resource_1: 5 }, { resource_1: 10 }]);
    second.resolve();
    await flush;
    assert.equal(statuses.at(-1), 'saved');
    saver.dispose();
});

test('save failure stays visible and explicit retry writes latest value', async () => {
    let attempts = 0;
    const statuses = [];
    const saver = new SerializedLoadoutSaver(async () => { if (++attempts === 1) throw new Error('conflict'); }, (status, error) => statuses.push([status, error?.message]), 1000);
    saver.initialize({});
    saver.change({ resource_1: 4 });
    await assert.rejects(saver.flush(), /conflict/);
    assert.deepEqual(statuses.at(-1), ['error', 'conflict']);
    await saver.flush();
    assert.equal(statuses.at(-1)[0], 'saved');
    assert.equal(attempts, 2);
    saver.dispose();
});

test('immediate Join waits for save, including after editor close', async () => {
    const request = deferred();
    const saver = new SerializedLoadoutSaver(() => request.promise, () => {}, 1000);
    saver.initialize({});
    saver.change({ resource_1: 5 });
    let joined = false;
    const join = saver.flush().then(() => { joined = true; });
    await Promise.resolve();
    assert.equal(joined, false);
    request.resolve();
    await join;
    assert.equal(joined, true);
    saver.dispose();
});

test('Join is blocked when the latest save fails', async () => {
    const saver = new SerializedLoadoutSaver(async () => { throw new Error('save rejected'); }, () => {}, 1000);
    saver.initialize({});
    saver.change({ resource_1: 1 });
    let joined = false;
    await assert.rejects(saver.flush().then(() => { joined = true; }), /save rejected/);
    assert.equal(joined, false);
    saver.dispose();
});
