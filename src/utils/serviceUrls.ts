const LOOPBACK_HOSTS = new Set(["localhost", "127.0.0.1", "::1"]);

function stripTrailingSlash(value: string): string {
    return value.endsWith("/") ? value.slice(0, -1) : value;
}

function getBrowserHostname(): string | null {
    if (typeof window === "undefined") {
        return null;
    }

    const hostname = window.location.hostname?.trim().toLowerCase();
    return hostname || null;
}

function getRuntimeProtocols(): { http: "http" | "https"; ws: "ws" | "wss" } {
    if (typeof window !== "undefined" && window.location.protocol === "https:") {
        return { http: "https", ws: "wss" };
    }

    return { http: "http", ws: "ws" };
}

function rewriteLoopbackUrl(rawUrl: string): string {
    try {
        const resolved = new URL(rawUrl);
        const browserHost = getBrowserHostname();
        if (
            !browserHost ||
            LOOPBACK_HOSTS.has(browserHost) ||
            !LOOPBACK_HOSTS.has(resolved.hostname.toLowerCase())
        ) {
            return stripTrailingSlash(rawUrl);
        }

        resolved.hostname = browserHost;
        if (
            (resolved.protocol === "ws:" || resolved.protocol === "wss:") &&
            getRuntimeProtocols().ws === "wss"
        ) {
            resolved.protocol = "wss:";
        }

        return stripTrailingSlash(resolved.toString());
    } catch {
        return stripTrailingSlash(rawUrl);
    }
}

function defaultHttpBase(port: number): string {
    const { http } = getRuntimeProtocols();
    const hostname = getBrowserHostname() ?? "localhost";
    return `${http}://${hostname}:${port}`;
}

function defaultWsUrl(port: number, path: string): string {
    const { ws } = getRuntimeProtocols();
    const hostname = getBrowserHostname() ?? "localhost";
    return `${ws}://${hostname}:${port}${path}`;
}

function resolveHttpBase(envValue: string | undefined, port: number): string {
    const configured = envValue?.trim();
    if (configured) {
        return rewriteLoopbackUrl(configured);
    }

    return defaultHttpBase(port);
}

function resolveWsUrl(
    envValue: string | undefined,
    port: number,
    path: string,
): string {
    const configured = envValue?.trim();
    if (configured) {
        return rewriteLoopbackUrl(configured);
    }

    return defaultWsUrl(port, path);
}

export const AUTH_BASE = resolveHttpBase(process.env.NEXT_PUBLIC_AUTH_BASE, 8000);
export const API_BASE = resolveHttpBase(process.env.NEXT_PUBLIC_API_BASE, 8001);
export const MATCHMAKING_BASE = resolveHttpBase(
    process.env.NEXT_PUBLIC_MATCHMAKING_BASE,
    8002,
);
export const WS_URL = resolveWsUrl(process.env.NEXT_PUBLIC_WS_URL, 8001, "/ws");
