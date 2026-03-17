"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "../contexts/AuthContext";
import MatchReadyModal from "./MatchReadyModal";

const API_MATCH =
    process.env.NEXT_PUBLIC_MATCHMAKING_BASE || "http://localhost:8002";
const prepSecondsFromEnv = Number(process.env.NEXT_PUBLIC_PREP_SECONDS || 15);
const PREP_REDIRECT_SECONDS = Number.isFinite(prepSecondsFromEnv)
    ? Math.max(5, Math.min(120, Math.floor(prepSecondsFromEnv)))
    : 15;
const PREP_REDIRECT_MS = PREP_REDIRECT_SECONDS * 1000;

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

                const isNewInstance = data.instance_id !== pendingInstanceId;

                // If user is already on the game page for this instance, clear pending and don't show
                let currentInstance: string | null = null;
                let currentPath: string | null = null;
                if (typeof window !== "undefined") {
                    currentPath = window.location.pathname;
                    currentInstance = new URLSearchParams(
                        window.location.search,
                    ).get("instance_id");
                }
                if (
                    currentPath?.startsWith("/game") &&
                    currentInstance === data.instance_id
                ) {
                    setPendingInstanceId(null);
                    setShowMatchModal(false);
                    return;
                }

                // If this instance was suppressed (user dismissed after join), set pending but don't show
                if (data.instance_id === suppressedInstanceId) {
                    setPendingInstanceId(data.instance_id);
                    if (isNewInstance)
                        setRedirectAtMs(Date.now() + PREP_REDIRECT_MS);
                    setShowMatchModal(false);
                    return;
                }

                // If the user suppressed showing modals until a certain time, respect it
                if (suppressedUntil && Date.now() < suppressedUntil) {
                    setPendingInstanceId(data.instance_id);
                    if (isNewInstance)
                        setRedirectAtMs(Date.now() + PREP_REDIRECT_MS);
                    setShowMatchModal(false);
                    return;
                }

                // Only set redirectAtMs when a new instance appears; don't reset on repeated events
                setPendingInstanceId(data.instance_id);
                if (isNewInstance) {
                    setRedirectAtMs(Date.now() + PREP_REDIRECT_MS);
                }
                // only show modal if user hasn't dismissed this same instance
                if (data.instance_id !== dismissedInstanceId) {
                    setShowMatchModal(true);
                } else {
                    setShowMatchModal(false);
                }
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
        dismissedInstanceId,
        suppressedInstanceId,
        suppressedUntil,
        pendingInstanceId,
    ]);

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
