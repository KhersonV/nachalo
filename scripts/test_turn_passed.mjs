import assert from "node:assert/strict";
import { readActiveUserFromTurnPassed } from "../src/utils/turnPassed.mjs";

for (const payload of [
    { userId: 7, turnNumber: 2 },
    { active_user: 7, userId: 7, turnNumber: 2 },
    { active_user: 7, turnNumber: 2 },
]) {
    const active_user = readActiveUserFromTurnPassed(payload);
    assert.equal(active_user, 7);
    assert.notEqual(active_user, undefined);
}
assert.equal(readActiveUserFromTurnPassed({ turnNumber: 2 }), null);
assert.equal(readActiveUserFromTurnPassed({ active_user: 0 }), null);
console.log("TURN_PASSED contract: PASS");
