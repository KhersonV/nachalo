"use client";

import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import { API_BASE } from "@/utils/serviceUrls";
import { isHexSnapshot, type HexSnapshot } from "@/types/HexGameTypes";
import HexGameController from "@/components/HexGameController";

function GameContent() {
  const instanceId = useSearchParams().get("instance_id") ?? "";
  const { user, isLoading } = useAuth();
  const [snapshot, setSnapshot] = useState<HexSnapshot | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!instanceId || isLoading || !user?.token) return;
    const controller = new AbortController();
    const path = instanceId.startsWith("hex-test-") ? "hex-test" : "hex";
    setSnapshot(null);
    setError("");
    void fetch(`${API_BASE}/game/${path}/${encodeURIComponent(instanceId)}`, {
      headers: { Authorization: `Bearer ${user.token}` },
      cache: "no-store",
      signal: controller.signal,
    }).then(async response => {
      if (response.status === 410) throw new Error("This match uses a retired map and cannot be resumed.");
      if (!response.ok) throw new Error(`Match unavailable (${response.status}).`);
      const data: unknown = await response.json();
      if (!isHexSnapshot(data) || data.instanceId !== instanceId || !data.player) throw new Error("Invalid HEX match snapshot.");
      setSnapshot(data);
    }).catch(reason => {
      if (!controller.signal.aborted) setError(String(reason instanceof Error ? reason.message : reason));
    });
    return () => controller.abort();
  }, [instanceId, isLoading, user?.token]);

  if (!instanceId) return <p role="alert">Missing match ID. <Link href="/mode">Return to modes</Link></p>;
  if (!isLoading && !user) return <p role="alert">Please sign in to open this match. <Link href="/login">Sign in</Link></p>;
  if (error) return <p role="alert">{error} <Link href="/mode">Return to modes</Link></p>;
  if (!snapshot) return <div role="status">Loading HEX match...</div>;
  return <HexGameController key={instanceId} instanceId={instanceId} initialSnapshot={snapshot} />;
}

export default function GamePage() {
  return <Suspense fallback={null}><GameContent /></Suspense>;
}
