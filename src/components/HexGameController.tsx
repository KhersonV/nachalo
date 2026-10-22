"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import dynamic from "next/dynamic";
import { useAuth } from "@/contexts/AuthContext";
import { hexDistance, hexKey, type HexCoord } from "@/utils/hexGrid";
import { type HexActor, type HexAttackPreview, type HexAttackResult, type HexCombatIndicator, type HexPlayerResult, type HexRoute, type HexSnapshot, type HexVisualMove } from "@/types/HexGameTypes";
import { hexAttackIndicators, hexMoveIndicators } from "@/utils/hexCombatIndicators";
import { useHexMatchSnapshot } from "@/hooks/useHexMatchSnapshot";
import type { ActionLogEntry, ActionLogCategory, ActionLogTone } from "@/types";
import HexMap from "./HexMap";
import Inventory from "./Inventory";
import ActionLog from "./ActionLog";
import HexMatchResults from "./HexMatchResults";
import hexStyles from "@/styles/HexGameController.module.css";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { formatHexMessage } from "@/i18n/hexGame";
import { toHexHudView } from "@/features/hexHud/hexHudView";
import { attackApproachCells } from "@/features/hexHud/approach";
import { HexBattleShell, HexTopBar, HexHeroPanel, HexTargetPanel, HexQuestPanel, HexParticipantsPanel, HexActionBar } from "@/features/hexHud/HexHudPanels";
import hudStyles from "@/features/hexHud/HexBattleShell.module.css";

const HexMap3D = dynamic(() => import("./HexMap3D"), { ssr: false });

type Pending = { key: string; commandId: string };
type Props = { instanceId: string; initialSnapshot: HexSnapshot };
type HexBarrelResult = { outcome: "damage" | "resource" | "artifact"; damage?: number; health: number; itemName?: string };
type HexItemResult = { itemType: string; itemId: number; health: number; energy: number; axis?: "q" | "r"; value?: number };

export default function HexGameController({ instanceId, initialSnapshot }: Props) {
  const { user } = useAuth();
  const { language, setLanguage, messages: t } = useGameLanguage();
  const { snapshot, refresh, base, unavailable, syncError, connection, questNotice, turnTimeoutNotice, presenceNotice, visualNotices } = useHexMatchSnapshot(instanceId, initialSnapshot, user?.token);
  const [selected, setSelected] = useState<HexCoord | null>(null);
  const [route, setRoute] = useState<HexRoute | null>(null);
  const [routeRevision, setRouteRevision] = useState<number | null>(null);
  const [targetId, setTargetId] = useState<number | null>(null);
	const [targetKind, setTargetKind] = useState<"player" | "monster">("monster");
	const [clockNow, setClockNow] = useState(Date.now());
  const [attackPreview, setAttackPreview] = useState<HexAttackPreview | null>(null);
  const [attackPreviewRevision, setAttackPreviewRevision] = useState<number | null>(null);
  const [previewFailure, setPreviewFailure] = useState<{ key: string; reason: string } | null>(null);
  const [approachPlan, setApproachPlan] = useState<{ key: string; destination: HexCoord; route: HexRoute } | null>(null);
  const [lastAttack, setLastAttack] = useState<HexAttackResult | null>(null);
  const [indicators, setIndicators] = useState<HexCombatIndicator[]>([]);
  const visualized = useRef(new Set<string>());
  const indicatorTimers = useRef<number[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [inventoryOpen, setInventoryOpen] = useState(false);
  const [logOpen, setLogOpen] = useState(false);
  const [mapMode, setMapMode] = useState<"2d" | "3d">("2d");
  const mapModeRef = useRef(mapMode);
  mapModeRef.current = mapMode;
  const connectionRef = useRef(connection);
  connectionRef.current = connection;
  const [visualMove, setVisualMove] = useState<HexVisualMove | null>(null);
  const [openedBarrel, setOpenedBarrel] = useState<{ commandId: string; instanceId: string; position: HexCoord } | null>(null);
  const [webglAvailable, setWebglAvailable] = useState(true);
  const [introVisible, setIntroVisible] = useState(false);
  const introStarted = useRef(false);
  const [blueprintKey, setBlueprintKey] = useState<string | null>(null);
  const [feedback, setFeedback] = useState("");
  const [log, setLog] = useState<ActionLogEntry[]>([]);
  const pendingMove = useRef<Pending | null>(null);
  const pendingAttack = useRef<Pending | null>(null);
  const pendingCollect = useRef<Pending | null>(null);
  const pendingBarrel = useRef<Pending | null>(null);
  const pendingItem = useRef<Pending | null>(null);
  const pendingTurn = useRef<Pending | null>(null);
  const pendingBuild = useRef<Pending | null>(null);
  const pendingStructureAttack = useRef<Pending | null>(null);
  const pendingExtract = useRef<Pending | null>(null);
  const routeRequest = useRef(0);
  const actionInFlight = useRef(false);

  useEffect(() => () => indicatorTimers.current.forEach(window.clearTimeout), []);
  useEffect(() => { if (connection !== "live") { setVisualMove(null); setOpenedBarrel(null); } }, [connection]);
  const showIndicators = useCallback((key: string, next: HexCombatIndicator[]) => {
    if (visualized.current.has(key) || next.length === 0) return;
    visualized.current.add(key);
    setIndicators(current => [...current, ...next]);
    const ids = new Set(next.map(item => item.id));
    const timer = window.setTimeout(() => {
      setIndicators(current => current.filter(item => !ids.has(item.id)));
      indicatorTimers.current = indicatorTimers.current.filter(item => item !== timer);
    }, 1100);
    indicatorTimers.current.push(timer);
  }, []);
  const showPlayerVitals = (key: string, health: number, energy?: number) => {
    if (!snapshot.player) return;
    const position = snapshot.player.position;
    const next: HexCombatIndicator[] = [];
    const hp = health - snapshot.player.health;
    const ep = energy === undefined ? 0 : energy - snapshot.player.energy;
    if (hp !== 0) next.push({ id: `${key}:hp`, position, text: `${hp > 0 ? "+" : ""}${hp} HP`, tone: hp > 0 ? "heal" : "damage", lane: 0 });
    if (ep !== 0) next.push({ id: `${key}:energy`, position, text: `${ep > 0 ? "+" : ""}${ep} энергии`, tone: ep > 0 ? "energyGain" : "energyLoss", lane: next.length });
    showIndicators(key, next);
  };
  useEffect(() => {
    for (const notice of visualNotices) {
      if (visualized.current.has(notice.commandId)) continue;
      if (notice.kind === "attack") showIndicators(notice.commandId, hexAttackIndicators(snapshot, notice, notice.commandId));
      if (notice.kind === "move" && notice.position) showIndicators(notice.commandId, hexMoveIndicators({ ...notice, position: notice.position }, notice.commandId));
    }
  }, [visualNotices, showIndicators, snapshot]);

  const record = useCallback((message: string, category: ActionLogCategory, tone: ActionLogTone = "info") => {
    setLog(current => [...current.slice(-29), { id: crypto.randomUUID(), timestamp: Date.now(), message, category, tone }]);
    if (tone === "success") setFeedback(message);
  }, []);
  useEffect(() => {
    if (!feedback) return;
    const timer = window.setTimeout(() => setFeedback(""), 3000);
    return () => window.clearTimeout(timer);
  }, [feedback]);
  useEffect(() => { if (syncError) setError(syncError); }, [syncError]);
  useEffect(() => {
    if (!initialSnapshot.questArtifact || initialSnapshot.finished || initialSnapshot.turnNumber !== 1 || initialSnapshot.stateRevision > 3) return;
    const key = `hex-artifact-intro:${instanceId}`;
    if (window.localStorage.getItem(key) && !introStarted.current) return;
    introStarted.current = true;
    window.localStorage.setItem(key, "shown");
    setIntroVisible(true);
    const timer = window.setTimeout(() => setIntroVisible(false), 3400);
    return () => window.clearTimeout(timer);
  }, [instanceId, initialSnapshot]);
  useEffect(() => { if (unavailable || snapshot.finished || !snapshot.player || snapshot.player.health <= 0) { setTargetId(null); setAttackPreview(null); setLastAttack(null); setSelected(null); setRoute(null); } }, [unavailable, snapshot.finished, snapshot.player?.health]);
  useEffect(() => { setRoute(null); setApproachPlan(null); }, [snapshot.stateRevision]);
	useEffect(() => { const timer = window.setInterval(() => setClockNow(Date.now()), 250); return () => window.clearInterval(timer); }, []);
  const post = useCallback(async <T,>(action: string, body: object): Promise<T> => {
    if (!user?.token) throw new Error(t.sessionUnavailable);
    const response = await fetch(`${base}/${action}`, {
      method: "POST", headers: { Authorization: `Bearer ${user.token}`, "Content-Type": "application/json" }, body: JSON.stringify(body),
    });
    if (!response.ok) throw new Error(await response.text());
    return response.json() as Promise<T>;
  }, [base, user?.token, t.sessionUnavailable]);
  const command = useCallback((ref: { current: Pending | null }, key: string) => {
    if (!ref.current || ref.current.key !== key) ref.current = { key, commandId: crypto.randomUUID() };
    return ref.current.commandId;
  }, []);

  const cellByCoord = useMemo(() => new Map(snapshot.cells.map(cell => [hexKey(cell.position), cell])), [snapshot.cells]);
  const selectedCell = selected ? cellByCoord.get(hexKey(selected)) : undefined;
  const currentCell = snapshot.player ? cellByCoord.get(hexKey(snapshot.player.position)) : undefined;
  const myTurn = !snapshot.finished && !snapshot.result && !unavailable && !!snapshot.player && snapshot.activeUser === snapshot.player.userId && snapshot.player.health > 0;
  const defeated = snapshot.result?.method === "defeat" || (!!snapshot.finished && !snapshot.result && !!snapshot.player && snapshot.player.health <= 0);
  const extracted = !!snapshot.result && snapshot.result.method !== "defeat";
  const questKey = `artifact_${snapshot.questArtifactId ?? 0}`;
  const hasQuest = !!snapshot.questArtifactId && Number(snapshot.player?.inventory?.[questKey]?.item_count ?? 0) > 0;
  const atPortal = !!currentCell?.isPortal && snapshot.mode !== "hex-test";
  const target = targetId === null ? undefined : snapshot.actors.find(actor => actor.id === `${targetKind}-${targetId}`);
  const targetKey = targetId === null ? "" : `${targetKind}:${targetId}:${snapshot.stateRevision}`;
  const contextAction = currentCell?.resource ? "collect" : currentCell?.barrel ? "open" : null;
  const structureAction = !!selectedCell?.structureType && !selectedCell.isUnderConstruction && !!snapshot.player && (selectedCell.structureOwnerUserId !== snapshot.player.userId || selectedCell.structureType === "wall");
  const view = toHexHudView(snapshot, selected, targetId, targetKind, attackPreviewRevision === snapshot.stateRevision ? attackPreview : null, routeRevision === snapshot.stateRevision ? route : null, clockNow, unavailable);

  useEffect(() => {
    if (selected && !view.cell) { setSelected(null); setRoute(null); }
    if (targetId !== null && !view.target) { setTargetId(null); setAttackPreview(null); setApproachPlan(null); setPreviewFailure(null); setSelected(null); setRoute(null); setLastAttack(null); }
  }, [snapshot.stateRevision, selected, targetId, view.cell, view.target]);

  useEffect(() => {
    if (targetId === null) return;
    if (!target || !myTurn) {
      setTargetId(null); setAttackPreview(null); setLastAttack(null);
      return;
    }
    let cancelled = false;
    void post<HexAttackPreview>("attack-preview", { targetId, targetKind }).then(preview => {
      if (!cancelled) { setAttackPreview(preview); setAttackPreviewRevision(snapshot.stateRevision); setPreviewFailure(null); setApproachPlan(null); setRoute(null); }
    }).catch(reason => {
      if (!cancelled) {
        setAttackPreview(null);
        setPreviewFailure({ key: `${targetKind}:${targetId}:${snapshot.stateRevision}`, reason: String(reason) });
        setError(String(reason));
      }
    });
    return () => { cancelled = true; };
  }, [targetId, targetKind, target, myTurn, snapshot.stateRevision, post]);

  useEffect(() => {
    if (!target || !snapshot.player || !myTurn || previewFailure?.key !== targetKey || actionInFlight.current || /energy/i.test(previewFailure.reason)) return;
    let cancelled = false;
    const findApproach = async () => {
      let best: { destination: HexCoord; route: HexRoute } | null = null;
      for (const destination of attackApproachCells(snapshot, target.position)) {
        if (cancelled) return;
        try {
          const candidate = await post<HexRoute>("route", { destination });
          const distance = hexDistance(destination, target.position);
          const attackCost = distance === 1 ? 6 : 8;
          if (candidate.affordable && candidate.energy - candidate.totalCost >= attackCost && (!best || candidate.totalCost < best.route.totalCost)) best = { destination, route: candidate };
        } catch { /* Another route may be legal; the server checks each candidate. */ }
      }
      if (cancelled || !best) return;
      setApproachPlan({ key: targetKey, ...best });
      setSelected(best.destination); setRoute(best.route); setRouteRevision(snapshot.stateRevision); setError("");
    };
    void findApproach();
    return () => { cancelled = true; };
  }, [previewFailure, targetKey, target, myTurn, snapshot, post]);

  const selectCell = async (position: HexCoord) => {
    if (busy || actionInFlight.current || !myTurn) return;
    if (blueprintKey) {
      const key = `${blueprintKey}:${hexKey(position)}`;
      actionInFlight.current = true; setBusy(true); setError("");
      try {
        await post("place-blueprint", { blueprintKey, position, commandId: command(pendingBuild, key) });
        await refresh(); pendingBuild.current = null; setBlueprintKey(null); setSelected(position);
        record(`${blueprintKey} (${position.q}, ${position.r})`, "effect", "success");
      } catch (reason) { setError(String(reason)); } finally { actionInFlight.current = false; setBusy(false); }
      return;
    }
    const occupant = snapshot.actors.find(actor => hexKey(actor.position) === hexKey(position));
    if (occupant && (occupant.kind === "monster" || occupant.kind === "player" && occupant.userId !== snapshot.player?.userId)) { selectMonster(occupant); return; }
    if (cellByCoord.get(hexKey(position))?.visibility === "unexplored") return;
    setTargetId(null); setAttackPreview(null); setApproachPlan(null); setPreviewFailure(null);
    const key = hexKey(position);
    if (snapshot.player && key === hexKey(snapshot.player.position)) { setSelected(position); setRoute(null); setError(""); return; }
    if (selected && hexKey(selected) === key && routeRevision === snapshot.stateRevision && route?.path.length && hexKey(route.path[route.path.length - 1]) === key) {
      if (!route.affordable) { setError(formatHexMessage(t.noEnergy, { cost: route.totalCost, energy: route.energy })); return; }
      actionInFlight.current = true; setBusy(true); setError("");
      let routeConfirmed = false;
      try {
        const moveCommandId = command(pendingMove, key);
        const moveIntent: HexVisualMove | null = mapMode === "3d" && snapshot.player
          ? { commandId: moveCommandId, instanceId, from: snapshot.player.position, position } : null;
        if (moveIntent) setVisualMove(moveIntent);
        const moved = await post<HexRoute>("move", { destination: position, commandId: moveCommandId });
        if (moveIntent) {
          routeConfirmed = Boolean(mapModeRef.current === "3d" && connectionRef.current === "live" && moved.path.length && hexKey(moved.path[moved.path.length - 1]) === hexKey(moved.position) &&
            moved.path.every((step, index) => hexDistance(index ? moved.path[index - 1] : moveIntent.from, step) === 1));
          setVisualMove(routeConfirmed ? { ...moveIntent, position: moved.position, path: moved.path, stateRevision: moved.stateRevision } : null);
        }
        showIndicators(moveCommandId, hexMoveIndicators(moved, moveCommandId));
        await refresh();
        pendingMove.current = null; setSelected(cellByCoord.get(key)?.barrel ? position : null); setRoute(null);
        record(`${t.moved} (${position.q}, ${position.r})`, "movement", "success");
      } catch (reason) { if (!routeConfirmed) setVisualMove(null); setError(String(reason)); } finally { actionInFlight.current = false; setBusy(false); }
      return;
    }
    const request = ++routeRequest.current;
    setSelected(position); setRoute(null); setError(""); pendingMove.current = null;
    try {
      const next = await post<HexRoute>("route", { destination: position });
      if (request !== routeRequest.current) return;
      setRoute(next); setRouteRevision(snapshot.stateRevision);
      if (!next.affordable) setError(formatHexMessage(t.noEnergy, { cost: next.totalCost, energy: next.energy }));
    } catch (reason) { if (request === routeRequest.current) setError(String(reason)); }
  };
  const selectMonster = (actor: HexActor) => {
    if (busy || actionInFlight.current || !myTurn || actor.kind === "player" && actor.teamId === snapshot.player?.teamId) return;
    const id = actor.kind === "player" ? actor.userId ?? 0 : Number(actor.id.replace(/^monster-/, ""));
    if (!Number.isSafeInteger(id)) return;
    if (targetId === id && targetKind === actor.kind) return;
    setSelected(null); setRoute(null); setApproachPlan(null); setPreviewFailure(null); setTargetId(id); setTargetKind(actor.kind); setAttackPreview(null); setLastAttack(null); setError(""); pendingAttack.current = null;
  };
  const approachAndAttack = async () => {
    const plan = approachPlan;
    if (!plan || plan.key !== targetKey || !plan.route.affordable || !target || targetId === null || busy || actionInFlight.current || !myTurn) return;
    const id = targetId, kind = targetKind;
    actionInFlight.current = true; setBusy(true); setError("");
    let routeConfirmed = false;
    try {
      const moveCommandId = command(pendingMove, hexKey(plan.destination));
      const moveIntent: HexVisualMove | null = mapMode === "3d" && snapshot.player
        ? { commandId: moveCommandId, instanceId, from: snapshot.player.position, position: plan.destination } : null;
      if (moveIntent) setVisualMove(moveIntent);
      const moved = await post<HexRoute>("move", { destination: plan.destination, commandId: moveCommandId });
      if (moveIntent) {
        routeConfirmed = Boolean(mapModeRef.current === "3d" && connectionRef.current === "live" && moved.path.length && hexKey(moved.path[moved.path.length - 1]) === hexKey(moved.position) &&
          moved.path.every((step, index) => hexDistance(index ? moved.path[index - 1] : moveIntent.from, step) === 1));
        setVisualMove(routeConfirmed ? { ...moveIntent, position: moved.position, path: moved.path, stateRevision: moved.stateRevision } : null);
      }
      pendingMove.current = null;
      showIndicators(moveCommandId, hexMoveIndicators(moved, moveCommandId));
      const fresh = await refresh();
      setApproachPlan(null); setSelected(null); setRoute(null);
      if (!fresh || fresh.stateRevision < (moved.stateRevision ?? 0) || fresh.activeUser !== fresh.player?.userId || !fresh.player || hexKey(fresh.player.position) !== hexKey(plan.destination) || fresh.player.health <= 0) throw new Error("Movement changed the match. Attack stopped; inspect the current position.");
      if (!fresh.actors.some(actor => actor.id === `${kind}-${id}` && actor.health !== 0)) throw new Error("Target is no longer visible. Attack stopped.");
      await post<HexAttackPreview>("attack-preview", { targetId: id, targetKind: kind });
      const attackCommandId = command(pendingAttack, `${kind}:${id}`);
      const result = await post<HexAttackResult>("attack", { targetId: id, targetKind: kind, commandId: attackCommandId });
      pendingAttack.current = null;
      showIndicators(attackCommandId, hexAttackIndicators(fresh, result, attackCommandId));
      setLastAttack(result);
      await refresh();
      record(`${kind === "player" ? t.player : t.monsterHit} ${id}: ${t.damage} ${result.damage}`, "attack", "success");
    } catch (reason) { if (!routeConfirmed) setVisualMove(null); setError(`Attack stopped: ${String(reason)}`); }
    finally { actionInFlight.current = false; setBusy(false); }
  };
  const attackTarget = async () => {
    if (approachPlan?.key === targetKey && !attackPreview) { await approachAndAttack(); return; }
    if (busy || actionInFlight.current || !myTurn || targetId === null || !target || !attackPreview || attackPreviewRevision !== snapshot.stateRevision) return;
    const id = targetId;
    actionInFlight.current = true; setBusy(true); setError("");
    try {
      const attackCommandId = command(pendingAttack, `${targetKind}:${id}`);
      const result = await post<HexAttackResult>("attack", { targetId: id, targetKind, commandId: attackCommandId });
      showIndicators(attackCommandId, hexAttackIndicators(snapshot, result, attackCommandId));
      pendingAttack.current = null;
      setLastAttack(result);
      if (result.monsterDied || result.newTargetHP <= 0 || result.newAttackerHP <= 0) {
        setTargetId(null); setAttackPreview(null);
      } else {
        setAttackPreview(previous => previous ? { ...previous, targetHP: result.newTargetHP, energy: result.energy } : previous);
      }
      await refresh();
      record(`${targetKind === "player" ? t.player : t.monsterHit} ${id}: ${t.damage} ${result.damage}, ${t.counterattack} ${result.counterDamage}${result.monsterDied || result.playerDied ? `, ${t.killed}` : ""}`, "attack", "success");
    } catch (reason) { setError(String(reason)); } finally { actionInFlight.current = false; setBusy(false); }
  };
  const attackStructure = async () => {
    if (!selectedCell?.structureType || selectedCell.isUnderConstruction || !myTurn || busy || actionInFlight.current) return;
    const position = selectedCell.position;
    actionInFlight.current = true; setBusy(true); setError("");
    try {
      const result = await post<{ damage: number; destroyed: boolean }>("attack-structure", { position, commandId: command(pendingStructureAttack, hexKey(position)) });
      await refresh(); pendingStructureAttack.current = null;
      record(`${selectedCell.structureType}: ${result.damage} ${t.damage}${result.destroyed ? `, ${t.killed}` : ""}`, "attack", "success");
    } catch (reason) { setError(String(reason)); } finally { actionInFlight.current = false; setBusy(false); }
  };
  const collect = async () => {
    if (busy || actionInFlight.current || !myTurn || contextAction !== "collect" || !currentCell?.resource) return;
    actionInFlight.current = true; setBusy(true); setError("");
    try {
      await post("collect", { position: currentCell.position, commandId: command(pendingCollect, hexKey(currentCell.position)) });
      await refresh();
      record(`${t.pickedUp}: ${currentCell.resource.type}`, "resource", "success");
      pendingCollect.current = null; setSelected(null); setRoute(null);
    } catch (reason) { setError(String(reason)); } finally { actionInFlight.current = false; setBusy(false); }
  };
  const openBarrel = async () => {
    if (busy || actionInFlight.current || !myTurn || contextAction !== "open" || !currentCell?.barrel) return;
    const position = currentCell.position;
    actionInFlight.current = true; setBusy(true); setError("");
    try {
      const barrelCommandId = command(pendingBarrel, hexKey(position));
      const result = await post<HexBarrelResult>("open-barrel", { position, commandId: barrelCommandId });
      if (mapModeRef.current === "3d" && connectionRef.current === "live") setOpenedBarrel({ commandId: barrelCommandId, instanceId, position });
      if (result.outcome === "damage") showPlayerVitals(barrelCommandId, result.health);
      await refresh(); pendingBarrel.current = null; setSelected(null); setRoute(null);
      record(result.outcome === "damage" ? `${t.barrelDamage} ${result.damage}` : `${result.outcome === "artifact" ? t.barrelArtifact : t.barrelResource} ${result.itemName ?? ""}`, "barrel", "success");
    } catch (reason) { setError(String(reason)); throw reason; } finally { actionInFlight.current = false; setBusy(false); }
  };
  const useItem = async (itemType: "resource" | "scroll", itemId: number): Promise<string> => {
    if (busy || actionInFlight.current || !myTurn) throw new Error(t.cannotUse);
    const key = `${itemType}_${itemId}`;
    actionInFlight.current = true; setBusy(true); setError("");
    try {
      const itemCommandId = command(pendingItem, key);
      const result = await post<HexItemResult>("use-item", { itemType, itemId, commandId: itemCommandId });
      showPlayerVitals(itemCommandId, result.health, result.energy);
      await refresh(); pendingItem.current = null;
      const message = result.axis ? `${t.scroll}: ${result.axis.toUpperCase()} = ${result.value}` : `${t.itemUsed} · HP ${result.health} · ${t.energy} ${result.energy}`;
      record(message, "resource", "success");
      return message;
    } catch (reason) { setError(String(reason)); throw reason; } finally { actionInFlight.current = false; setBusy(false); }
  };
  const endTurn = async () => {
    if (busy || actionInFlight.current || !myTurn || !snapshot.turnNumber) return;
    actionInFlight.current = true; setBusy(true); setError("");
    try {
      await post("end-turn", { expectedTurn: snapshot.turnNumber, commandId: command(pendingTurn, String(snapshot.turnNumber)) });
      await refresh();
      record(`${t.turn} ${snapshot.turnNumber} · ${t.turnEnded}`, "turn", "success");
      pendingTurn.current = null; setSelected(null); setRoute(null);
    } catch (reason) { setError(String(reason)); } finally { actionInFlight.current = false; setBusy(false); }
  };

  const extract = async () => {
    if (busy || actionInFlight.current || !myTurn || !atPortal) return;
    const method = hasQuest ? "artifact" : "paid";
    if (method === "paid") {
      if ((snapshot.player?.balance ?? 0) < 50) { setError(t.needGold); return; }
      if (!window.confirm(t.confirmPaid)) return;
    }
    actionInFlight.current = true; setBusy(true); setError("");
    try {
      await post<HexPlayerResult>("extract", { method, confirmedPaid: method === "paid", commandId: command(pendingExtract, method) });
      await refresh(); pendingExtract.current = null; setSelected(null); setRoute(null); setTargetId(null); setAttackPreview(null);
      record(method === "artifact" ? t.extractedArtifact : t.extractedPaid, "portal", "success");
    } catch (reason) { setError(String(reason)); } finally { actionInFlight.current = false; setBusy(false); }
  };

  const quest = snapshot.questArtifact;
  const questStatus = quest?.status === "carried" && quest.position ? formatHexMessage(t.questCarried, { player: quest.carrierName || `${t.player} ${quest.carrierUserId}`, q: quest.position.q, r: quest.position.r })
    : quest?.status === "ground" && quest.position ? formatHexMessage(t.questGround, quest.position)
    : quest?.status === "extracted" ? t.questExtracted : quest?.status === "found" ? t.questFoundStatus : t.questUnknown;

  return <div className={hexStyles.hexGame} data-geometry="hex-v1">
    <HexBattleShell
      top={<HexTopBar snapshot={snapshot} view={view} connection={connection} language={language} onLanguage={setLanguage} labels={t} />}
      left={<HexHeroPanel view={view} labels={t} language={language} logOpen={logOpen} onLog={() => setLogOpen(open => !open)} />}
      right={<>
        <HexQuestPanel quest={quest} status={questStatus} labels={t} />
        {!view.finished && <HexTargetPanel view={view} lastAttack={lastAttack} approach={approachPlan?.key === targetKey ? approachPlan.route : undefined} onCloseTarget={() => { setTargetId(null); setAttackPreview(null); setApproachPlan(null); setPreviewFailure(null); setLastAttack(null); }} labels={t} />}
        <HexParticipantsPanel snapshot={snapshot} labels={t} />
      </>}
      bottom={<HexActionBar view={view} busy={busy} inventoryOpen={inventoryOpen} contextAction={contextAction} atPortal={atPortal} hasQuest={hasQuest} approachReady={approachPlan?.key === targetKey} structureAction={structureAction} idleActionLabel={{ en: "Action", ru: "Действие", de: "Aktion" }[language]} bagLabel={{ en: "Backpack", ru: "Рюкзак", de: "Rucksack" }[language]}
        onAttack={() => void attackTarget()} onStructureAttack={() => void attackStructure()} onInventory={() => setInventoryOpen(open => !open)} onContext={() => contextAction === "collect" ? void collect() : void openBarrel().catch(() => {})} onExtract={() => void extract()} onEndTurn={() => void endTurn()} labels={t} />}
    >
      <div className={hudStyles.mapModeSwitch} role="group" aria-label="Map view">
        <button type="button" aria-pressed={mapMode === "2d"} onClick={() => { setVisualMove(null); setOpenedBarrel(null); setMapMode("2d"); }}>2D</button>
        <button type="button" aria-pressed={mapMode === "3d"} disabled={!webglAvailable} onClick={() => setMapMode("3d")}>3D</button>
      </div>
      {mapMode === "3d" ? <HexMap3D snapshot={{ instanceId: snapshot.instanceId, cells: snapshot.cells, edges: snapshot.edges, actors: snapshot.actors, player: snapshot.player, questArtifact: snapshot.questArtifact }} visualMove={visualMove} openedBarrel={openedBarrel} connection={connection} visualNotices={visualNotices} selected={view.finished ? null : view.cell?.position ?? null} route={view.finished ? null : view.route ?? null} selectedTargetId={view.finished || !view.target ? null : targetId} selectedTargetKind={targetKind} indicators={indicators} busy={busy || !myTurn} labels={t}
        onCellClick={position => void selectCell(position)} onMonsterClick={actor => void selectMonster(actor)} onUnavailable={() => { setWebglAvailable(false); setMapMode("2d"); setError("WebGL is unavailable. Switched to 2D map."); }} />
        : <HexMap snapshot={{ cells: snapshot.cells, edges: snapshot.edges, actors: snapshot.actors, player: snapshot.player }} selected={view.finished ? null : view.cell?.position ?? null} route={view.finished ? null : view.route ?? null} selectedTargetId={view.finished || !view.target ? null : targetId} selectedTargetKind={targetKind} indicators={indicators} busy={busy || !myTurn} labels={t}
          onCellClick={position => void selectCell(position)} onMonsterClick={actor => void selectMonster(actor)} />}
    </HexBattleShell>
    {blueprintKey && <button type="button" className={hudStyles.placementCancel} onClick={() => { setBlueprintKey(null); pendingBuild.current = null; }}>{t.close} · {blueprintKey}</button>}
    {!view.finished && view.remaining >= 1 && view.remaining <= 5 && <div key={`${snapshot.turnNumber}:${view.remaining}`} className={hudStyles.finalSecond} aria-live="assertive">{view.remaining}</div>}
    {(introVisible && quest || questNotice) && <div key={questNotice ? `found:${questNotice.userId}:${questNotice.q}:${questNotice.r}` : `intro:${instanceId}`} className={hudStyles.artifactFlight} role="status">
      <div className={hudStyles.artifactFlightHead}>{quest?.image && <img src={quest.image} alt="" />}<span><small>{t.questArtifact}</small><strong>{quest?.name || t.questArtifact}</strong></span></div>
      <p>{questNotice ? formatHexMessage(t.questAnnouncement, { artifact: quest?.name || t.questArtifact, player: questNotice.playerName || `${t.player} ${questNotice.userId}`, q: questNotice.q, r: questNotice.r }) : t.questGoal}</p>
    </div>}
    {logOpen && (log.length ? <ActionLog entries={log} className={hudStyles.logSheet} labels={t} /> : <p className={hudStyles.logEmpty}>{t.noEvents}</p>)}
    {feedback && <p role="status" className={hudStyles.feedback}>{feedback}</p>}
    <div className={hudStyles.noticeStack}>
      {turnTimeoutNotice && <p role="status" className={hudStyles.notice}>{t.turnTimedOut}</p>}
      {presenceNotice && <p role="status" className={hudStyles.notice}>{formatHexMessage(presenceNotice.type === "disconnected" ? t.playerDisconnected : presenceNotice.type === "reconnected" ? t.playerReconnected : presenceNotice.type === "exit" ? t.playerLeftPortal : t.playerLightning, { userId: presenceNotice.userId })}</p>}
      {snapshot.disconnectedPlayers?.map(player => <p role="status" className={hudStyles.notice} key={player.userId}>{formatHexMessage(t.reconnectDeadline, { userId: player.userId, seconds: Math.max(0, Math.ceil((Date.parse(player.deadline) - clockNow) / 1000)) })}</p>)}
    </div>
    {error && <p role="alert" className={`${hudStyles.feedback} ${hudStyles.error}`}>{error}</p>}
    {inventoryOpen && <Inventory className={hudStyles.inventorySheet} onRequestClose={() => setInventoryOpen(false)} hexInventory={snapshot.player?.inventory ?? {}} onHexUseItem={useItem} onHexBlueprintPlacementStart={key => { setBlueprintKey(key); setInventoryOpen(false); setRoute(null); }} hexBusy={busy || !myTurn} hexLabels={t} />}
    {(defeated || extracted) && <HexMatchResults snapshot={snapshot} labels={t} />}
  </div>;
}
