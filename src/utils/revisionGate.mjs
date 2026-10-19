/** Decide whether a committed patch can follow the last applied snapshot/patch. */
export function decideRevision(current, incoming) {
    if (!Number.isSafeInteger(incoming) || incoming < 0) return "resync";
    if (current === null || !Number.isSafeInteger(current)) return "resync";
    if (incoming <= current) return "duplicate";
    if (incoming !== current + 1) return "resync";
    return "apply";
}
