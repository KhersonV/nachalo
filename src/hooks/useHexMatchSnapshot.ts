"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { timeHexStage } from "@/features/hexLoading/performance";
import { API_BASE, WS_URL } from "@/utils/serviceUrls";
import { isHexSnapshot, type HexSnapshot, type HexVisualNotice } from "@/types/HexGameTypes";

export function useHexMatchSnapshot(instanceId: string, initialSnapshot: HexSnapshot, token: string | undefined) {
  const [connection, setConnection] = useState<"connecting" | "live" | "reconnecting">("connecting");
  const [snapshot, setSnapshot] = useState(initialSnapshot);
  const [syncError, setSyncError] = useState("");
  const [unavailable, setUnavailable] = useState(false);
  const [visualNotices, setVisualNotices] = useState<HexVisualNotice[]>([]);
  const [questNotice, setQuestNotice] = useState<{ userId: number; playerName?: string; q: number; r: number } | null>(null);
  const [turnTimeoutNotice, setTurnTimeoutNotice] = useState(false);
	const [presenceNotice, setPresenceNotice] = useState<{ type: "disconnected" | "reconnected" | "lightning" | "exit"; userId: number } | null>(null);
	useEffect(() => { if (!turnTimeoutNotice) return; const timer = window.setTimeout(() => setTurnTimeoutNotice(false), 5000); return () => window.clearTimeout(timer); }, [turnTimeoutNotice]);
	useEffect(() => { if (!presenceNotice) return; const timer = window.setTimeout(() => setPresenceNotice(null), 8000); return () => window.clearTimeout(timer); }, [presenceNotice]);
  useEffect(() => {
    if (!questNotice) return;
    const timer = window.setTimeout(() => setQuestNotice(null), 8000);
    return () => window.clearTimeout(timer);
  }, [questNotice]);
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
      return next;
    }
  }, [base, instanceId, token]);

  useEffect(() => {
    if (!token) return;
    const finishConnect = timeHexStage("WebSocket connect + authoritative refresh");
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
        window.dispatchEvent(new Event("nachalo:chat-socket-open"));
        socket?.send(JSON.stringify({ type: "JOIN_MATCH", instanceId }));
        void refresh().then(() => { if (!closed && socket?.readyState === WebSocket.OPEN) { finishConnect(); setConnection("live"); } }).catch(report);
      };
      socket.onmessage = event => {
        try {
          const message = JSON.parse(event.data);
          if (message?.type === "presence.join" || message?.type === "presence.leave") {
            window.dispatchEvent(new CustomEvent("nachalo:presence-event", { detail: message }));
            return;
          }
          if (message?.type === "chat.message" || message?.type === "chat.system") {
            window.dispatchEvent(new CustomEvent("nachalo:chat-message", { detail: message.payload }));
            return;
          }
          const payload = message?.payload;
          if (payload?.instanceId !== instanceId || payload?.schemaVersion !== 2) return;
		  if ((message.type === "PLAYER_DISCONNECTED" || message.type === "PLAYER_RECONNECTED") && Number.isSafeInteger(payload.userId)) {
			setPresenceNotice({ type: message.type === "PLAYER_DISCONNECTED" ? "disconnected" : "reconnected", userId: payload.userId });
			void refresh().catch(report);
			return;
		  }
          if (message.type === "MATCH_HEX_VISUAL" && typeof payload.commandId === "string" && (payload.kind === "attack" || payload.kind === "move")) {
            setVisualNotices(current => [...current.slice(-24), payload as HexVisualNotice]);
            return;
          }
          if (message.type === "MATCH_PATCH" && Array.isArray(payload.changes)) {
			if (payload.changes.some((change: {type?: string}) => change.type === "TURN_TIMEOUT")) setTurnTimeoutNotice(true);
			if (payload.changes.some((change: {type?: string}) => change.type === "PLAYER_LIGHTNING")) {
			  const lightning = payload.changes.find((change: {type?: string; payload?: {userId?: number}}) => change.type === "PLAYER_LIGHTNING");
			  setPresenceNotice({ type: "lightning", userId: Number(lightning?.payload?.userId) || 0 });
			}
			const exit = payload.changes.find((change: {type?: string; payload?: {userId?: number}}) => change.type === "PLAYER_LEFT_PORTAL");
			if (exit) setPresenceNotice({ type: "exit", userId: Number(exit.payload?.userId) || 0 });
            const found = payload.changes.find((change: { type?: string }) => change.type === "QUEST_ARTIFACT_FOUND")?.payload;
            if (found && Number.isSafeInteger(found.userId) && Number.isSafeInteger(found.position?.q) && Number.isSafeInteger(found.position?.r)) {
              setQuestNotice({ userId: found.userId, playerName: typeof found.playerName === "string" ? found.playerName : undefined, q: found.position.q, r: found.position.r });
            }
          }
          if (message.type === "MATCH_UPDATE" && isHexSnapshot(payload) && payload.player && payload.stateRevision >= revision.current) {
            revision.current = payload.stateRevision;
            setSnapshot(payload);
          } else if (message.type === "MATCH_PATCH" && payload.stateRevision > revision.current) {
            void refresh().catch(report);
          }
        } catch { /* Ignore unrelated socket frames. */ }
      };
      socket.onclose = () => { if (!closed) { setConnection("reconnecting"); timer = setTimeout(connect, 1500); } };
      socket.onerror = () => { if (!closed) setConnection("reconnecting"); };
    };
    connect();
    return () => { closed = true; if (timer) clearTimeout(timer); socket?.close(); };
  }, [instanceId, refresh, token]);

  return { snapshot, refresh, base, unavailable, syncError, connection: unavailable ? "unavailable" as const : connection, questNotice, turnTimeoutNotice, presenceNotice, visualNotices };
}
