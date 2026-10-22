import test from 'node:test';
import assert from 'node:assert/strict';
import { DEFAULT_MATCH_FILTER, filterMatches, hasOverallProgress, modeFamily, personalResult, representedClasses } from './historyView.ts';

const entry = (id, mode, characterType, status) => ({ matchId: id, mode, characterType, playerResultSummary: { status } });

test('personal result uses player status, not match winner', () => {
    assert.deepEqual(personalResult({ status: 'eliminated', isWinner: false, survived: false }), { label: 'Eliminated', tone: 'danger', key: 'eliminated' });
    assert.equal(personalResult({ status: 'winner', isWinner: false }).label, 'Victory');
    assert.equal(personalResult({ status: 'unknown', isWinner: true }).label, 'Result unavailable');
});

test('mode and local filters only use known modes and loaded rows', () => {
    const rows = [entry('one', 'PVE', 'guardian', 'winner'), entry('two', '2x2', 'ranger', 'eliminated'), entry('three', 'mystery', 'mystic', 'survived')];
    assert.equal(modeFamily('2x2'), 'pvp');
    assert.equal(modeFamily('mystery'), null);
    assert.deepEqual(filterMatches(rows, { mode: 'pvp', heroClass: 'ranger', result: 'eliminated' }).map(row => row.matchId), ['two']);
    assert.deepEqual(filterMatches(rows, DEFAULT_MATCH_FILTER), rows);
    assert.deepEqual(filterMatches([], DEFAULT_MATCH_FILTER), []);
});

test('only played classes appear; missing overall values do not become zero', () => {
    const summary = { matchesPlayed: 1, wins: 0, losses: 1, winRate: 0, classes: [{ characterType: 'guardian', matchesPlayed: 0 }, { characterType: 'ranger', matchesPlayed: 1 }] };
    assert.deepEqual(representedClasses(summary).map(row => row.characterType), ['ranger']);
    assert.equal(hasOverallProgress(summary), true);
    assert.equal(hasOverallProgress({ ...summary, wins: undefined }), false);
});
