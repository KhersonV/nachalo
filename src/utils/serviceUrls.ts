function stripTrailingSlash(value: string): string {
    return value.endsWith("/") ? value.slice(0, -1) : value;
}

function getWsOrigin(): string {
    if (typeof window !== "undefined") {
        return window.location.origin.replace(/^http/, "ws");
    }
    return "ws://localhost";
}

function resolveBase(envValue: string | undefined): string {
    const configured = envValue?.trim();
    if (!configured) return "";
    return stripTrailingSlash(configured);
}

function resolveWs(envValue: string | undefined): string {
    const configured = envValue?.trim();

    if (!configured) {
        return `${getWsOrigin()}/ws`;
    }

    if (configured.startsWith("/")) {
        return `${getWsOrigin()}${configured}`;
    }

    return stripTrailingSlash(configured);
}

export const AUTH_BASE = resolveBase(process.env.NEXT_PUBLIC_AUTH_BASE);
export const API_BASE = resolveBase(process.env.NEXT_PUBLIC_API_BASE);
export const MATCHMAKING_BASE = resolveBase(process.env.NEXT_PUBLIC_MATCHMAKING_BASE);
export const WS_URL = resolveWs(process.env.NEXT_PUBLIC_WS_URL);
