import test from 'node:test';
import assert from 'node:assert/strict';
import { matchesProfileView, ownedHeroes, performConfirmedSocialAction, profileViewId, relationActions, xpProgress } from './profileView.ts';
import { normalizeAvatarPath } from '../../utils/normalizeAvatarPath.ts';

test('public view only accepts the matching loaded player', () => {
    assert.equal(profileViewId('42'), 42);
    assert.equal(profileViewId(null), null);
    assert.equal(matchesProfileView(42, 42), true);
    assert.equal(matchesProfileView(42, 7), false);
    assert.equal(matchesProfileView(null, 7), false);
});

test('relation allows only server-backed actions and never self actions', () => {
    assert.deepEqual(relationActions('none', 2, 1), ['send']);
    assert.deepEqual(relationActions('friend', 2, 1), ['remove']);
    assert.deepEqual(relationActions('incoming', 2, 1), ['accept', 'decline']);
    assert.deepEqual(relationActions('outgoing', 2, 1), ['cancel']);
    assert.deepEqual(relationActions('none', 1, 1), []);
});

test('roster includes only owned enabled heroes; XP uses the server threshold', () => {
    const heroes = [{ id: 'ranger', owned: false, enabled: true, sortOrder: 1 }, { id: 'guardian', owned: true, enabled: true, sortOrder: 0 }, { id: 'mystic', owned: true, enabled: false, sortOrder: 2 }];
    assert.deepEqual(ownedHeroes(heroes).map(hero => hero.id), ['guardian']);
    assert.equal(xpProgress(125, 500), 25);
    assert.equal(xpProgress(125, undefined), null);
});

test('avatar fallback preserves a supplied player image', () => {
    assert.equal(normalizeAvatarPath(''), '/guardian/guardian.webp');
    assert.equal(normalizeAvatarPath('/custom/account.webp'), '/custom/account.webp');
});

test('social refresh runs after confirmation and never after a rejected action', async () => {
    const events = [];
    await performConfirmedSocialAction(async () => { events.push('request'); }, async () => { events.push('refresh'); });
    assert.deepEqual(events, ['request', 'refresh']);
    await assert.rejects(performConfirmedSocialAction(async () => { throw new Error('denied'); }, async () => { events.push('bad refresh'); }), /denied/);
    assert.deepEqual(events, ['request', 'refresh']);
});
