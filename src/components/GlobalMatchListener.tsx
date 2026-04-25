"use client";

import React, { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { MATCHMAKING_BASE as API_MATCH } from "@/utils/serviceUrls";
import { useAuth } from "../contexts/AuthContext";
import MatchReadyModal from "./MatchReadyModal";
const prepSecondsFromEnv = Number(process.env.NEXT_PUBLIC_PREP_SECONDS || 15);
const PREP_REDIRECT_SECONDS = Number.isFinite(prepSecondsFromEnv)
    ? Math.max(5, Math.min(120, Math.floor(prepSecondsFromEnv)))
    : 15;
const PREP_REDIRECT_MS = PREP_REDIRECT_SECONDS * 1000;

type CurrentMatchResponse = {
    instance_id?: string;
    instanceId?: string;
};

function withBust(url: string) {
    const separator = url.includes("?") ? "&" : "?";
    return `${url}${separator}_=${Date.now()}`;
}

export default function GlobalMatchListener() {
    const { user } = useAuth();
    const router = useRouter();
    const [pendingInstanceId, setPendingInstanceId] = useState<string | null>(
        null,
    );
    const [redirectAtMs, setRedirectAtMs] = useState<number | null>(null);
    const [showMatchModal, setShowMatchModal] = useState(false);
    const [dismissedInstanceId, setDismissedInstanceId] = useState<
        string | null
    >(null);
    const [suppressedInstanceId, setSuppressedInstanceId] = useState<
        string | null
    >(null);
    const [suppressedUntil, setSuppressedUntil] = useState<number | null>(null);

    const storageKeyFor = (userId: number | string | undefined) =>
        userId
            ? `dismissedPendingInstance:${userId}`
            : "dismissedPendingInstance";
    const suppressedUntilKeyFor = (userId: number | string | undefined) =>
        userId ? `suppressedPendingUntil:${userId}` : "suppressedPendingUntil";

    const handlePendingMatch = useCallback(
        (instanceId: string | null | undefined) => {
            if (!instanceId) return;

            const now = Date.now();
            const isNewInstance = instanceId !== pendingInstanceId;

            if (typeof window !== "undefined") {
                const currentPath = window.location.pathname;
                const currentInstance = new URLSearchParams(
                    window.location.search,
                ).get("instance_id");

                if (
                    currentPath.startsWith("/game") &&
                    currentInstance === instanceId
                ) {
                    setPendingInstanceId(null);
                    setRedirectAtMs(null);
                    setShowMatchModal(false);
                    return;
                }
            }

            setPendingInstanceId(instanceId);
            if (isNewInstance || !redirectAtMs || redirectAtMs <= now) {
                setRedirectAtMs(now + PREP_REDIRECT_MS);
            }

            const suppressionActive =
                suppressedUntil !== null && now < suppressedUntil;

            if (
                instanceId === suppressedInstanceId ||
                suppressionActive ||
                instanceId === dismissedInstanceId
            ) {
                setShowMatchModal(false);
                return;
            }

            setShowMatchModal(true);
        },
        [
            dismissedInstanceId,
            pendingInstanceId,
            redirectAtMs,
            suppressedInstanceId,
            suppressedUntil,
        ],
    );

    // restore dismissed/suppressed ids from localStorage so refresh doesn't re-show modal
    useEffect(() => {
        if (!user?.id) return;
        try {
            const key = storageKeyFor(user.id);
            const val = localStorage.getItem(key);
            if (val) setDismissedInstanceId(val);
            const supKey = `suppressedPendingInstance:${user.id}`;
            const supVal = localStorage.getItem(supKey);
            if (supVal) setSuppressedInstanceId(supVal);
            const supUntilKey = suppressedUntilKeyFor(user.id);
            const until = localStorage.getItem(supUntilKey);
            if (until) {
                const n = Number(until);
                if (!Number.isNaN(n)) setSuppressedUntil(n);
            }
        } catch (e) {
            // ignore storage errors
        }
    }, [user?.id]);

    useEffect(() => {
        if (!user?.token || !user?.id) return;

        const sourceUrl = `${API_MATCH}/matchmaking/stream?player_id=${user.id}&token=${encodeURIComponent(
            user.token,
        )}`;
        let es: EventSource | null = null;

        try {
            es = new EventSource(sourceUrl);
        } catch (e) {
            console.error("Failed to create EventSource", e);
            return;
        }

        es.onmessage = (ev) => {
            try {
                const data = JSON.parse(ev.data);
                if (!data || !data.instance_id) return;

                handlePendingMatch(data.instance_id);
            } catch (e) {
                console.error("Invalid SSE message", e);
            }
        };

        es.onerror = (err) => {
            console.error("SSE error", err);
            // EventSource auto-reconnects; if we get a fatal error, close and let effect recreate on deps change
        };

        return () => {
            if (es) {
                es.close();
            }
        };
    }, [
        user?.token,
        user?.id,
        handlePendingMatch,
    ]);

    useEffect(() => {
        if (!user?.token || !user?.id) return;

        let alive = true;
        const checkCurrentMatch = async () => {
            try {
                const res = await fetch(
                    withBust(
                        `${API_MATCH}/matchmaking/currentMatch?player_id=${user.id}`,
                    ),
                    {
                        headers: {
                            Authorization: `Bearer ${user.token}`,
                        },
                        cache: "no-store",
                    },
                );
                if (!res.ok) return;

                const data: CurrentMatchResponse = await res.json();
                if (!alive) return;

                handlePendingMatch(data.instance_id ?? data.instanceId);
            } catch (e) {
                console.error("currentMatch fallback poll failed", e);
            }
        };

        checkCurrentMatch();
        const intervalId = window.setInterval(checkCurrentMatch, 3000);

        return () => {
            alive = false;
            window.clearInterval(intervalId);
        };
    }, [handlePendingMatch, user?.id, user?.token]);

    useEffect(() => {
        if (!pendingInstanceId || !redirectAtMs) return;
        const delay = Math.max(0, redirectAtMs - Date.now());
        const t = setTimeout(() => {
            router.push(`/game?instance_id=${pendingInstanceId}`);
        }, delay);
        return () => clearTimeout(t);
    }, [pendingInstanceId, redirectAtMs, router]);

    if (!user?.token) return null;

    const handleClose = () => {
        // persist that user dismissed this instance so refresh won't re-open
        try {
            if (pendingInstanceId && user?.id) {
                const key = storageKeyFor(user.id);
                localStorage.setItem(key, pendingInstanceId);
                setDismissedInstanceId(pendingInstanceId);
            }
        } catch (e) {
            // ignore
        }
        // hide modal but suppress re-show until redirect time (so it won't re-open repeatedly)
        const until = redirectAtMs ?? Date.now() + PREP_REDIRECT_MS;
        try {
            if (user?.id) {
                const supUntilKey = suppressedUntilKeyFor(user.id);
                localStorage.setItem(supUntilKey, String(until));
            }
        } catch (e) {}
        setSuppressedUntil(until);
        setShowMatchModal(false);
    };

    const handleJoin = (id: string) => {
        try {
            if (user?.id) {
                // suppress re-showing this instance after join
                const supKey = `suppressedPendingInstance:${user.id}`;
                localStorage.setItem(supKey, id);
                setSuppressedInstanceId(id);
                // also clear dismissed key if present
                const key = storageKeyFor(user.id);
                localStorage.removeItem(key);
                setDismissedInstanceId(null);
            }
        } catch (e) {
            // ignore
        }
        setPendingInstanceId(null);
        setRedirectAtMs(null);
        setShowMatchModal(false);
        router.push(`/game?instance_id=${id}`);
    };

    // removed navigation effect — check() already inspects window.location each poll

    return (
        <>
            {pendingInstanceId && showMatchModal && (
                <MatchReadyModal
                    instanceId={pendingInstanceId}
                    redirectAtMs={redirectAtMs}
                    onClose={handleClose}
                    onJoin={handleJoin}
                />
            )}
        </>
    );
}
