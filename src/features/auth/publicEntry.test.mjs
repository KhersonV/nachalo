import test from 'node:test';
import assert from 'node:assert/strict';
import { entryDestination, registrationPayload } from './publicEntry.ts';

test('entry waits for session restoration and sends authenticated players to Lobby', () => {
  assert.equal(entryDestination(null, true), null);
  assert.equal(entryDestination(null, false), '/login');
  assert.equal(entryDestination({ id: 7 }, false), '/mode');
});

test('registration keeps the existing auth request shape', () => {
  assert.deepEqual(registrationPayload({ email: 'a@example.com', password: 'secret1', name: 'A' }, { id: 'guardian', image: '/guardian/guardian.webp' }), {
    email: 'a@example.com', password: 'secret1', name: 'A', image: '/guardian/guardian.webp', characterType: 'guardian',
  });
});
