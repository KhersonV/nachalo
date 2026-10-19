// Both fields are accepted while older backend instances still emit userId.
export function readActiveUserFromTurnPassed(payload) {
    const userId = payload?.active_user ?? payload?.userId;
    return Number.isInteger(userId) && userId > 0 ? userId : null;
}
