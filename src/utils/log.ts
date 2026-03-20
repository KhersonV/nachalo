// Lightweight debug logger — no-ops outside development builds
export function debugLog(...args: any[]) {
    try {
        if (typeof window === "undefined") return;
        if (process.env.NODE_ENV === "development") {
            // prefer console.debug for dev-only logs
            // eslint-disable-next-line no-console
            console.debug(...args);
        }
    } catch (e) {
        // swallow
    }
}

export default debugLog;
