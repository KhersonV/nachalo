import assert from "node:assert/strict";
import { decideRevision } from "../src/utils/revisionGate.mjs";

// A reconnect snapshot resets the cursor to the database's committed revision.
let cursor = 4;
assert.equal(decideRevision(cursor, 5), "apply");
cursor = 5;
assert.equal(decideRevision(cursor, 5), "duplicate");
assert.equal(decideRevision(cursor, 7), "resync");
cursor = 7; // fresh snapshot after the gap
assert.equal(decideRevision(cursor, 8), "apply");
assert.equal(decideRevision(null, 1), "resync");
console.log("revision gate: PASS");
