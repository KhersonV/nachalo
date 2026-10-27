"use client";

import { Suspense, useEffect, useRef, useState, type ComponentType } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import { API_BASE } from "@/utils/serviceUrls";
import { isHexSnapshot, type HexSnapshot } from "@/types/HexGameTypes";
import { loadHexScreen } from "@/features/hexLoading/preload";
import HexLoading from "@/features/hexLoading/HexLoading";
import { preloadHexImages, COMMON_HEX_IMAGES } from "@/features/hexLoading/resources";
import { heroArt, monsterArt, resourceArt } from "@/utils/hexArt";
import { timeHexStage } from "@/features/hexLoading/performance";

function GameContent() {
  const instanceId = useSearchParams().get("instance_id") ?? "";
  const { user, isLoading } = useAuth();
  const [snapshot, setSnapshot] = useState<HexSnapshot | null>(null);
  const [error, setError] = useState("");
  const [attempt, setAttempt] = useState(0);
  const [Screen, setScreen] = useState<ComponentType<{ instanceId: string; initialSnapshot: HexSnapshot; onInteractive?: () => void }> | null>(null);
  const interactiveTimer = useRef<(() => void) | null>(null);
  useEffect(() => {
    interactiveTimer.current = timeHexStage("entry to first interactive field");
  }, [instanceId, attempt]);

  useEffect(() => {
    if (!instanceId || isLoading || !user?.token) return;
    const controller = new AbortController();
    const finishSnapshot = timeHexStage("match state HTTP + JSON");
    const codeTimeout = window.setTimeout(() => {
      if (!controller.signal.aborted) setError("Battle code is taking too long to load. Please retry.");
    }, 30_000);
    void loadHexScreen().then(module => {
      window.clearTimeout(codeTimeout);
      if (!controller.signal.aborted) setScreen(() => module.default);
    }).catch(() => {
      window.clearTimeout(codeTimeout);
      if (!controller.signal.aborted) setError("Battle code could not be loaded. Please retry.");
    });
    void preloadHexImages(COMMON_HEX_IMAGES, controller.signal);
    const path = instanceId.startsWith("hex-test-") ? "hex-test" : "hex";
    setSnapshot(null);
    setError("");
    const stateTimeout = window.setTimeout(() => {
      setError("Match state is taking too long to load. Please retry.");
      controller.abort();
    }, 30_000);
    void fetch(`${API_BASE}/game/${path}/${encodeURIComponent(instanceId)}`, {
      headers: { Authorization: `Bearer ${user.token}` },
      cache: "no-store",
      signal: controller.signal,
    }).then(async response => {
      if (response.status === 410) throw new Error("This match uses a retired map and cannot be resumed.");
      if (!response.ok) throw new Error(`Match unavailable (${response.status}).`);
      const data: unknown = await response.json();
      if (!isHexSnapshot(data) || data.instanceId !== instanceId || !data.player) throw new Error("Invalid HEX match snapshot.");
      window.clearTimeout(stateTimeout);
      if (controller.signal.aborted) return;
      finishSnapshot();
      setSnapshot(data);
      // Only artwork visible in this snapshot; never retain authoritative match data.
      const artwork = [heroArt(data.player?.characterType), ...data.actors.map(actor => actor.kind === "monster" ? monsterArt(actor) : heroArt(undefined)),
        ...data.cells.filter(cell => cell.visibility !== "unexplored").map(cell => resourceArt(cell.resource))];
      void preloadHexImages(artwork.filter((url): url is string => !!url), controller.signal);
    }).catch(reason => {
      window.clearTimeout(stateTimeout);
      if (!controller.signal.aborted) setError(String(reason instanceof Error ? reason.message : reason));
    });
    return () => {
      controller.abort();
      window.clearTimeout(codeTimeout);
      window.clearTimeout(stateTimeout);
    };
  }, [instanceId, isLoading, user?.token, attempt]);

  if (!instanceId) return <p role="alert">Missing match ID. <Link href="/mode">Return to modes</Link></p>;
  if (!isLoading && !user) return <p role="alert">Please sign in to open this match. <Link href="/login">Sign in</Link></p>;
  if (error) return <HexLoading error={error} onRetry={() => setAttempt(value => value + 1)} />;
  if (!snapshot || snapshot.instanceId !== instanceId || !Screen) return <HexLoading stage={Screen ? "map" : "preparing"} />;
  return <Screen key={instanceId} instanceId={instanceId} initialSnapshot={snapshot} onInteractive={() => interactiveTimer.current?.()} />;
}

export default function GamePage() {
  return <Suspense fallback={<HexLoading />}><GameContent /></Suspense>;
}
