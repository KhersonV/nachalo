"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { API_BASE, WS_URL } from "@/utils/serviceUrls";
import { isHexSnapshot, type HexSnapshot } from "@/types/HexGameTypes";

export function useHexMatchSnapshot(instanceId: string, initialSnapshot: HexSnapshot, token: string | undefined) {
  const [snapshot, setSnapshot] = useState(initialSnapshot);
  const [syncError, setSyncError] = useState("");
  const [unavailable, setUnavailable] = useState(false);
  const revision = useRef(initialSnapshot.stateRevision);
  const base = `${API_BASE}/game/${initialSnapshot.mode === "hex-test" ? "hex-test" : "hex"}/${encodeURIComponent(instanceId)}`;

  const refresh = useCallback(async () => {
    if (!token) return;
    const response = await fetch(base, { headers: { Authorization: `Bearer ${token}` }, cache: "no-store" });
    if (response.status === 404 || response.status === 410) setUnavailable(true);
    if (!response.ok) throw new Error(`HEX snapshot ${response.status}: ${await response.text()}`);
    const next: unknown = await response.json();
    if (!isHexSnapshot(next) || !next.player || next.instanceId !== instanceId) throw new Error("Invalid HEX snapshot");
    if (next.stateRevision >= revision.current) {
      revision.current = next.stateRevision;
      setSnapshot(next);
      setUnavailable(false);
      setSyncError("");
    }
  }, [base, instanceId, token]);

  useEffect(() => {
    if (!token) return;
    let closed = false;
    let socket: WebSocket | null = null;
    let timer: ReturnType<typeof setTimeout> | null = null;
    const report = (reason: unknown) => setSyncError(String(reason));
    const connect = () => {
      const url = new URL(WS_URL);
      url.searchParams.set("token", token);
      url.searchParams.set("instanceId", instanceId);
      socket = new WebSocket(url.toString());
      socket.onopen = () => {
        socket?.send(JSON.stringify({ type: "JOIN_MATCH", instanceId }));
        void refresh().catch(report);
      };
      socket.onmessage = event => {
        try {
          const message = JSON.parse(event.data);
          const payload = message?.payload;
          if (payload?.instanceId !== instanceId || payload?.schemaVersion !== 2) return;
          if (message.type === "MATCH_UPDATE" && isHexSnapshot(payload) && payload.player && payload.stateRevision >= revision.current) {
            revision.current = payload.stateRevision;
            setSnapshot(payload);
          } else if (message.type === "MATCH_PATCH" && payload.stateRevision > revision.current) {
            void refresh().catch(report);
          }
        } catch { /* Ignore unrelated socket frames. */ }
      };
      socket.onclose = () => { if (!closed) timer = setTimeout(connect, 1500); };
    };
    connect();
    return () => { closed = true; if (timer) clearTimeout(timer); socket?.close(); };
  }, [instanceId, refresh, token]);

  return { snapshot, refresh, base, unavailable, syncError };
}
