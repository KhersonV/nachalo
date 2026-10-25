"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import dynamic from "next/dynamic";
import { useAuth } from "@/contexts/AuthContext";
import { hexDistance, hexKey, type HexCoord } from "@/utils/hexGrid";
import { type HexActor, type HexAttackPreview, type HexAttackResult, type HexEngagementResult, type HexCombatIndicator, type HexPlayerResult, type HexRoute, type HexSnapshot, type HexVisualMove } from "@/types/HexGameTypes";
import { hexAttackIndicators, hexMoveIndicators } from "@/utils/hexCombatIndicators";
import { useHexMatchSnapshot } from "@/hooks/useHexMatchSnapshot";
import type { ActionLogEntry, ActionLogCategory, ActionLogTone } from "@/types";
import HexMap from "./HexMap";
import Inventory from "./Inventory";
import ActionLog from "./ActionLog";
import HexMatchResults from "./HexMatchResults";
import hexStyles from "@/styles/HexGameController.module.css";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";
import { formatHexMessage } from "@/i18n/hexGame";
import { toHexHudView } from "@/features/hexHud/hexHudView";
import { resolveSmartAction, type SmartAction } from "@/features/hexHud/smartAction";
import SmartActionButton from "@/features/hexHud/SmartActionButton";
import HexActionNotices, { useActionNotices } from "@/features/hexHud/HexActionNotices";
import { HexBattleShell, HexTopBar, HexHeroPanel, HexTargetPanel, HexQuestPanel, HexParticipantsPanel, HexActionBar } from "@/features/hexHud/HexHudPanels";
import DraggableHudPanel, { useHudPanel, useDesktopHud } from "@/features/hexHud/DraggableHudPanel";
import hudStyles from "@/features/hexHud/HexBattleShell.module.css";

const equipmentDropNotice = (result: HexAttackResult, ownerUserId: number | undefined, label: string) =>
  result.drops?.filter(drop => !ownerUserId || drop.ownerUserId === ownerUserId).map(drop => `${label}: ${drop.name}`).join(", ") ?? "";

const HexMap3D = dynamic(() => import("./HexMap3D"), { ssr: false });

type Pending = { key: string; commandId: string };
type Props = { instanceId: string; initialSnapshot: HexSnapshot };
type HexBarrelResult = { outcome: "damage" | "resource" | "artifact"; damage?: number; health: number; itemName?: string };
type HexItemResult = { itemType: string; itemId: number; health: number; budget?: import("@/types/HexGameTypes").HexBudgets; axis?: "q" | "r"; value?: number };

export default function HexGameController({ instanceId, initialSnapshot }: Props) {
  const { user } = useAuth();
  const { language, setLanguage, messages: t } = useGameLanguage();
  const { snapshot, refresh, base, unavailable, syncError, connection, questNotice, turnTimeoutNotice, presenceNotice, visualNotices } = useHexMatchSnapshot(instanceId, initialSnapshot, user?.token);
  const selectedPanel = useHudPanel("selected");
  const desktopHud = useDesktopHud();
  const [detailsRequested, setDetailsRequested] = useState(false);
  const snapshotRef = useRef(snapshot);
  snapshotRef.current = snapshot;
  const { notices, notify } = useActionNotices();
  const setError = useCallback((message: string) => { if (message) notify(message, "error"); }, [notify]);
  const [selected, setSelected] = useState<HexCoord | null>(null);
  const [route, setRoute] = useState<HexRoute | null>(null);
  const [routeRevision, setRouteRevision] = useState<number | null>(null);
  const [targetId, setTargetId] = useState<number | null>(null);
	const [targetKind, setTargetKind] = useState<"player" | "monster">("monster");
	const [clockNow, setClockNow] = useState(Date.now());
  const [attackPreview, setAttackPreview] = useState<HexAttackPreview | null>(null);
  const [attackPreviewRevision, setAttackPreviewRevision] = useState<number | null>(null);
  const [previewFailure, setPreviewFailure] = useState<{ key: string; reason: string } | null>(null);
  const [lastAttack, setLastAttack] = useState<HexAttackResult | null>(null);
  const [indicators, setIndicators] = useState<HexCombatIndicator[]>([]);
  const visualized = useRef(new Set<string>());
  const indicatorTimers = useRef<number[]>([]);
  const [busy, setBusy] = useState(false);
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
  const [log, setLog] = useState<ActionLogEntry[]>([]);
  const pendingMove = useRef<Pending | null>(null);
  const pendingAttack = useRef<Pending | null>(null);
  const pendingCollect = useRef<Pending | null>(null);
  const pendingBarrel = useRef<Pending | null>(null);
  const pendingItem = useRef<Pending | null>(null);
  const pendingExchange = useRef<Pending | null>(null);
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
  const showPlayerVitals = (key: string, health: number) => {
    if (!snapshot.player) return;
    const position = snapshot.player.position;
    const next: HexCombatIndicator[] = [];
    const hp = health - snapshot.player.health;
    if (hp !== 0) next.push({ id: `${key}:hp`, position, text: `${hp > 0 ? "+" : ""}${hp} HP`, tone: hp > 0 ? "heal" : "damage", lane: 0 });
    showIndicators(key, next);
  };
  const previousBudget = useRef<{ revision: number; ap: number; mp: number; reactions: number } | null>(null);
  useEffect(() => {
    const hero = snapshot.player;
    if (!hero) return;
    const before = previousBudget.current;
    if (before && before.revision !== snapshot.stateRevision) {
      const changes: HexCombatIndicator[] = [];
      for (const [label, delta] of [["AP", (hero.ap - before.ap) / 2], ["MP", (hero.mp - before.mp) / 2], ["Reaction", hero.reactions - before.reactions]] as const) {
        if (delta) changes.push({ id: `${snapshot.stateRevision}:${label}`, position: hero.position, text: `${delta > 0 ? "+" : ""}${delta} ${label}`, tone: delta > 0 ? "energyGain" : "energyLoss", lane: changes.length });
      }
      if (changes.length) showIndicators(`budget:${snapshot.stateRevision}`, changes);
    }
    previousBudget.current = { revision: snapshot.stateRevision, ap: hero.ap, mp: hero.mp, reactions: hero.reactions };
  }, [snapshot.stateRevision, snapshot.player, showIndicators]);
  useEffect(() => {
    for (const notice of visualNotices) {
      if (visualized.current.has(notice.commandId)) continue;
      if (notice.kind === "attack") showIndicators(notice.commandId, hexAttackIndicators(snapshot, notice, notice.commandId));
      if (notice.kind === "move" && notice.position) showIndicators(notice.commandId, hexMoveIndicators({ ...notice, position: notice.position }, notice.commandId));
    }
  }, [visualNotices, showIndicators, snapshot]);

  useEffect(() => { if (turnTimeoutNotice) notify(t.turnTimedOut); }, [turnTimeoutNotice, notify, t.turnTimedOut]);
  useEffect(() => {
    if (presenceNotice) notify(formatHexMessage(presenceNotice.type === "disconnected" ? t.playerDisconnected : presenceNotice.type === "reconnected" ? t.playerReconnected : presenceNotice.type === "exit" ? t.playerLeftPortal : t.playerLightning, { userId: presenceNotice.userId }));
  }, [presenceNotice, notify, t]);
  useEffect(() => {
    if (questNotice) notify(formatHexMessage(t.questAnnouncement, { artifact: snapshot.questArtifact?.name || t.questArtifact, player: questNotice.playerName || `${t.player} ${questNotice.userId}`, q: questNotice.q, r: questNotice.r }));
  }, [questNotice, notify, t, snapshot.questArtifact?.name]);
  const record = useCallback((message: string, category: ActionLogCategory, tone: ActionLogTone = "info") => {
    setLog(current => [...current.slice(-29), { id: crypto.randomUUID(), timestamp: Date.now(), message, category, tone }]);
    if (tone === "success" && category !== "attack" && category !== "movement") notify(message);
  }, [notify]);
  useEffect(() => { if (syncError) setError(syncError); }, [syncError, setError]);
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
  useEffect(() => { if (snapshot.finished || !snapshot.player || snapshot.player.health <= 0) { setTargetId(null); setAttackPreview(null); setLastAttack(null); setSelected(null); setRoute(null); } }, [snapshot.finished, snapshot.player?.health]);
	useEffect(() => { const timer = window.setInterval(() => setClockNow(Date.now()), 250); return () => window.clearInterval(timer); }, []);
  const post = useCallback(async <T,>(action: string, body: object): Promise<T> => {
    if (!user?.token) throw new Error(t.sessionUnavailable);
    const response = await fetch(`${base}/${action}`, {
      method: "POST", headers: { Authorization: `Bearer ${user.token}`, "Content-Type": "application/json" }, body: JSON.stringify(body),
    });
    if (!response.ok) {
      const error = new Error(uiText(language, (await response.text()).trim()));
      throw Object.assign(error, { status: response.status });
    }
    return response.json() as Promise<T>;
  }, [base, user?.token, t.sessionUnavailable, language]);
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
  const contextAction = currentCell?.resource && (currentCell.resource.item_type !== "artifact" || !currentCell.barrel) ? "collect" : currentCell?.barrel ? "open" : null;
  const view = toHexHudView(snapshot, selected, targetId, targetKind, attackPreviewRevision === snapshot.stateRevision ? attackPreview : null, routeRevision === snapshot.stateRevision ? route : null, clockNow, unavailable);

  useEffect(() => {
    if (selected && !view.cell) { setSelected(null); setRoute(null); }
    if (targetId !== null && !view.target) { setTargetId(null); setAttackPreview(null); setPreviewFailure(null); setLastAttack(null); }
  }, [snapshot.stateRevision, selected, targetId, view.cell, view.target]);

  useEffect(() => {
    ++routeRequest.current;
    setRoute(null); setRouteRevision(null);
    setAttackPreview(null); setAttackPreviewRevision(null); setPreviewFailure(null);
  }, [snapshot.stateRevision, snapshot.mapRevision, snapshot.activeUser, snapshot.player?.position.q, snapshot.player?.position.r]);

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
    if (occupant && (occupant.kind === "monster" || occupant.kind === "player" && occupant.userId !== snapshot.player?.userId)) { await selectMonster(occupant); return; }
    if (cellByCoord.get(hexKey(position))?.visibility === "unexplored") return;
    setTargetId(null); setAttackPreview(null); setPreviewFailure(null);
    const key = hexKey(position);
    if (snapshot.player && key === hexKey(snapshot.player.position)) { setSelected(position); setRoute(null); setError(""); return; }
    if (selected && hexKey(selected) === key && routeRevision === snapshot.stateRevision && route?.path.length && hexKey(route.path[route.path.length - 1]) === key) {
      if ((route.affordableSteps ?? 0) === 0) { setError("Недостаточно MP даже для первого шага"); return; }
      actionInFlight.current = true; setBusy(true); setError("");
      let routeConfirmed = false;
      try {
        const moveCommandId = command(pendingMove, `${key}:${route.previewToken}`);
        const moveIntent: HexVisualMove | null = mapMode === "3d" && snapshot.player
          ? { commandId: moveCommandId, instanceId, from: snapshot.player.position, position } : null;
        if (moveIntent) setVisualMove(moveIntent);
        const moved = await post<HexRoute>("move", { destination: position, previewToken: route.previewToken, commandId: moveCommandId });
        if (moveIntent) {
          routeConfirmed = Boolean(mapModeRef.current === "3d" && connectionRef.current === "live" && moved.path.length && hexKey(moved.path[moved.path.length - 1]) === hexKey(moved.position) &&
            moved.path.every((step, index) => hexDistance(index ? moved.path[index - 1] : moveIntent.from, step) === 1));
          setVisualMove(routeConfirmed ? { ...moveIntent, position: moved.position, path: moved.path, stateRevision: moved.stateRevision } : null);
        }
        showIndicators(moveCommandId, hexMoveIndicators(moved, moveCommandId));
        await refresh();
        pendingMove.current = null; setSelected(cellByCoord.get(hexKey(moved.position))?.barrel ? moved.position : null); setRoute(null);
        record(`${t.moved} (${moved.position.q}, ${moved.position.r})`, "movement", "success");
      } catch (reason) {
        if (!routeConfirmed) setVisualMove(null);
        if (typeof reason === "object" && reason && "status" in reason && Number(reason.status) >= 400 && Number(reason.status) < 500) { setRoute(null); setRouteRevision(null); pendingMove.current = null; void refresh().catch(() => {}); }
        setError(String(reason));
      } finally { actionInFlight.current = false; setBusy(false); }
      return;
    }
    const request = ++routeRequest.current;
    setSelected(position); setRoute(null); setError(""); pendingMove.current = null;
    try {
      const next = await post<HexRoute>("route", { destination: position });
      if (request !== routeRequest.current || snapshotRef.current.stateRevision !== snapshot.stateRevision || next.stateRevision !== snapshot.stateRevision) { void refresh().catch(() => {}); return; }
      setRoute(next); setRouteRevision(snapshot.stateRevision);
      if (!next.affordable) notify((next.affordableSteps ?? 0) > 0 ? "Повторное нажатие — пройти доступную часть маршрута" : "Недостаточно MP даже для первого шага");
    } catch (reason) { if (request === routeRequest.current) setError(String(reason)); }
  };
  const selectMonster = async (actor: HexActor) => {
    if (busy || actionInFlight.current || (actor.health ?? 0) <= 0 || actor.userId === snapshot.player?.userId || actor.kind === "player" && snapshot.player?.teamId !== undefined && actor.teamId === snapshot.player.teamId) return;
    const id = actor.kind === "player" ? actor.userId : Number(actor.id.replace(/^monster-/, ""));
    if (id === undefined || !Number.isSafeInteger(id)) return;
    const action = resolveSmartAction({ snapshot, language, selected: null, target: actor, pending: busy, unavailable,
      preview: targetId === id && targetKind === actor.kind && attackPreviewRevision === snapshot.stateRevision ? attackPreview ?? undefined : undefined,
      failure: previewFailure?.key === `${actor.kind}:${id}:${snapshot.stateRevision}` ? previewFailure.reason : undefined });
    ++routeRequest.current;
    if (targetId !== id || targetKind !== actor.kind) {
      setSelected(null); setRoute(null); setRouteRevision(null);
      setPreviewFailure(null); setAttackPreview(null); setLastAttack(null);
      pendingAttack.current = null;
    }
    setTargetId(id); setTargetKind(actor.kind);
    await performSmartAction(action);
  };
  const attackTarget = async (actor: HexActor) => {
    if (busy || actionInFlight.current || !myTurn) return;
    const id = actor.kind === "player" ? actor.userId : Number(actor.id.replace(/^monster-/, ""));
    if (id === undefined || !Number.isSafeInteger(id)) return;
    const kind = actor.kind;
    const confirmed = targetId === id && targetKind === kind && attackPreviewRevision === snapshot.stateRevision && attackPreview?.previewToken ? attackPreview : null;
    actionInFlight.current = true; setBusy(true);
    try {
      if (!confirmed) {
        const request = ++routeRequest.current;
        const revision = snapshot.stateRevision;
        const preview = await post<HexAttackPreview>("attack-preview", { targetId: id, targetKind: kind });
        if (request !== routeRequest.current || snapshotRef.current.stateRevision !== revision || preview.previewRevision !== revision) { void refresh().catch(() => {}); return; }
        setAttackPreview(preview); setAttackPreviewRevision(revision); setPreviewFailure(null);
        setRoute(preview.route ?? null); setRouteRevision(preview.route ? revision : null);
        setSelected(preview.route ? preview.attackPosition ?? null : null);
        if (preview.reason) notify(preview.reason);
        return;
      }
      if (confirmed.ap < confirmed.cost) { setError("Недостаточно AP для атаки"); return; }
      if (!confirmed.canAttack && !(confirmed.route?.affordableSteps)) { setError(confirmed.reason || "Атака недоступна"); return; }
      const attackCommandId = command(pendingAttack, `${kind}:${id}:${confirmed.previewToken}`);
      const from = snapshot.player!.position;
      const result = await post<HexEngagementResult>("attack", { targetId: id, targetKind: kind, previewToken: confirmed.previewToken, commandId: attackCommandId });
      if (result.movement) {
        const moved = result.movement;
        const visualId = `${attackCommandId}:movement`;
        showIndicators(visualId, hexMoveIndicators(moved, visualId));
        if (mapModeRef.current === "3d" && connectionRef.current === "live" && moved.path.length) setVisualMove({ commandId: visualId, instanceId, from, position: moved.position, path: moved.path, stateRevision: result.stateRevision });
      }
      if (result.attacked) {
        showIndicators(attackCommandId, hexAttackIndicators(snapshot, result, attackCommandId));
        setLastAttack(result);
        record(`${kind === "player" ? t.player : t.monsterHit} ${id}: ${t.damage} ${result.damage}, ${t.counterattack} ${result.counterDamage}${result.monsterDied || result.playerDied ? `, ${t.killed}` : ""}`, "attack", "success");
        const dropNotice = equipmentDropNotice(result, snapshot.player?.userId, t.categories.equipment);
        if (dropNotice) record(dropNotice, "equipment", "success");
      } else {
        setLastAttack(null);
        record(result.reason || "Выполнено частичное сближение", "movement", "success");
      }
      pendingAttack.current = null;
      setAttackPreview(null); setAttackPreviewRevision(null); setRoute(null); setRouteRevision(null); setSelected(null);
      if (result.monsterDied || result.newTargetHP <= 0 || result.newAttackerHP <= 0) setTargetId(null);
      await refresh();
    } catch (reason) {
      if (typeof reason === "object" && reason && "status" in reason && Number(reason.status) >= 400 && Number(reason.status) < 500) {
        setAttackPreview(null); setAttackPreviewRevision(null); setRoute(null); setRouteRevision(null); pendingAttack.current = null; void refresh().catch(() => {});
      }
      setError(String(reason));
    } finally { actionInFlight.current = false; setBusy(false); }
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
      const result = await post<{ itemName?: string; professionGathering?: boolean; professionXP?: number }>("collect", { position: currentCell.position, commandId: command(pendingCollect, hexKey(currentCell.position)) });
      await refresh();
      record(result.professionGathering ? formatHexMessage(uiText(language, "Gathered {material} ×1 · +{xp} profession XP"), { material: uiText(language, result.itemName || currentCell.resource.type), xp: result.professionXP ?? 0 }) : `${t.pickedUp}: ${currentCell.resource.type}`, "resource", "success");
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
  const useItem = async (itemType: "resource" | "scroll", itemId: number, targetUserId?: number): Promise<string> => {
    if (busy || actionInFlight.current || !myTurn) throw new Error(t.cannotUse);
    const entry = snapshot.player?.inventory?.[`${itemType}_${itemId}`];
    if (itemType === "resource" && (entry?.name || entry?.item_name || "").trim().toLowerCase() === "water" && waterReason) throw new Error(waterReason);
    const key = `${itemType}_${itemId}:${targetUserId ?? 0}`;
    actionInFlight.current = true; setBusy(true); setError("");
    try {
      const itemCommandId = command(pendingItem, key);
      const result = await post<HexItemResult>("use-item", { itemType, itemId, targetUserId, commandId: itemCommandId });
      showPlayerVitals(itemCommandId, result.health);
      await refresh(); pendingItem.current = null;
      const message = result.axis ? `${t.scroll}: ${result.axis.toUpperCase()} = ${result.value}` : `${t.itemUsed} · HP ${result.health} · AP ${(result.budget?.ap ?? 0) / 2}`;
      record(message, "resource", "success");
      return message;
    } catch (reason) { setError(String(reason)); throw reason; } finally { actionInFlight.current = false; setBusy(false); }
  };
  const exchangeAP = async () => {
    if (busy || actionInFlight.current || !myTurn) return;
    actionInFlight.current = true; setBusy(true); setError("");
    try {
      await post("exchange-ap", { commandId: command(pendingExchange, String(snapshot.player?.personalTurn)) });
      await refresh(); pendingExchange.current = null;
      record("−1 AP / +2 MP", "effect", "success");
    } catch (reason) { setError(String(reason)); }
    finally { actionInFlight.current = false; setBusy(false); }
  };
  const endTurn = async () => {
    if (busy || actionInFlight.current || !myTurn || !snapshot.turnNumber) return;
    actionInFlight.current = true; setBusy(true); setError("");
    try {
      await post("end-turn", { expectedTurn: snapshot.turnNumber, commandId: command(pendingTurn, String(snapshot.turnNumber)) });
      await refresh();
      record(`${t.turn} ${snapshot.turnNumber} · ${t.turnEnded}`, "turn", "success");
      pendingTurn.current = null;
    } catch (reason) { setError(String(reason)); } finally { actionInFlight.current = false; setBusy(false); }
  };

  const extract = async () => {
    if (busy || actionInFlight.current || !myTurn || !atPortal) return;
    if ((snapshot.player?.mp ?? 0) < 2) { setError(`Недостаточно ОП: нужно 1, доступно ${(snapshot.player?.mp ?? 0) / 2}`); return; }
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

  const smartAction = resolveSmartAction({ snapshot, language, selected, target, pending: busy, unavailable,
    preview: attackPreviewRevision === snapshot.stateRevision ? attackPreview ?? undefined : undefined,
    failure: previewFailure?.key === targetKey ? previewFailure.reason : undefined });
  const performSmartAction = async (action: SmartAction) => {
    if (actionInFlight.current || busy) return;
    if (!action.enabled) { if (action.kind !== "none") setError(action.disabledReason); return; }
    switch (action.kind) {
      case "attack": if (action.target && "kind" in action.target) await attackTarget(action.target); break;
      case "structure": await attackStructure(); break;
      case "collect": case "pickup": await collect(); break;
      case "barrel": await openBarrel().catch(() => {}); break;
      case "portal": await extract(); break;
    }
  };
  const handleCellClick = async (position: HexCoord) => {
    if (busy || actionInFlight.current) return;
    const cell = cellByCoord.get(hexKey(position));
    if (!blueprintKey && snapshot.player && cell && hexKey(position) === hexKey(snapshot.player.position) && (cell.resource || cell.barrel || cell.isPortal)) {
      ++routeRequest.current; setSelected(position); setTargetId(null); setAttackPreview(null);
      await performSmartAction(resolveSmartAction({ snapshot, language, selected: position, pending: busy, unavailable }));
      return;
    }
    await selectCell(position);
  };
  const waterEntries = Object.entries(snapshot.player?.inventory ?? {}).filter(([key, entry]) => /^resource_[1-9]\d*$/.test(key) && (entry.name || entry.item_name || "").trim().toLowerCase() === "water" && (entry.item_count ?? 0) > 0);
  const waterCount = waterEntries.reduce((sum, [, entry]) => sum + (entry.item_count ?? 0), 0);
  const waterTurns = Math.max(0, (snapshot.player?.waterAvailableTurn ?? 0) - (snapshot.player?.personalTurn ?? 0));
  const waterReason = busy ? "Выполняется действие…" : !myTurn ? t.waiting : waterCount === 0 ? "В инвентаре нет воды" : waterTurns > 0 ? `Вода доступна через ${waterTurns} личн. ход.` : (snapshot.player?.ap ?? 0) >= (snapshot.player?.maxAP ?? 0) ? "AP уже полные" : "";
  const drinkWater = async () => {
    if (actionInFlight.current || busy) return;
    if (waterReason) { setError(waterReason); return; }
    const water = waterEntries[0];
    if (water) await useItem("resource", Number(water[0].slice("resource_".length))).catch(() => {});
  };

  return <div className={hexStyles.hexGame} data-geometry="hex-v1">
    <HexBattleShell
      top={<HexTopBar snapshot={snapshot} view={view} connection={connection} language={language} onLanguage={setLanguage} labels={t} />}
      left={<HexHeroPanel view={view} labels={t} language={language} logOpen={logOpen} onLog={() => setLogOpen(open => !open)} water={{ count: waterCount, turns: waterTurns, reason: waterReason, onUse: () => void drinkWater() }} actor={snapshot.actors.find(actor => actor.userId === snapshot.player?.userId && actor.kind === "player")} journal={log.length ? <ActionLog entries={log} className={hudStyles.embeddedLog} expanded onExpandedChange={setLogOpen} labels={t} /> : <p>{t.noEvents}</p>} />}
      right={<>
        <HexQuestPanel quest={quest} status={questStatus} labels={t} />
        {!view.finished && detailsRequested && (view.target || view.cell) && <DraggableHudPanel panel={selectedPanel} title={view.target ? view.target.name || view.target.id : `${view.cell?.resource?.type || (view.cell?.barrel ? t.barrel : view.cell?.isPortal ? t.portal : t.cell)} (${view.cell?.position.q}, ${view.cell?.position.r})`} anchor="target" width={304} enabled={desktopHud}>
          <HexTargetPanel view={view} lastAttack={lastAttack} onCloseTarget={desktopHud ? undefined : () => setDetailsRequested(false)} labels={t} />
        </DraggableHudPanel>}
        <HexParticipantsPanel snapshot={snapshot} labels={t} />
      </>}
      bottom={<HexActionBar view={view} busy={busy} inventoryOpen={inventoryOpen} bagLabel={{ en: "Backpack", ru: "Рюкзак", de: "Rucksack" }[language]}
        smartAction={<SmartActionButton action={smartAction} onAction={() => void performSmartAction(smartAction)} />}
        canInspect={!!view.target || !!view.cell} onInspect={() => { setDetailsRequested(true); selectedPanel.setMode("expanded"); }}
        onInventory={() => setInventoryOpen(open => !open)} onEndTurn={() => void endTurn()} onExchange={() => void exchangeAP()} labels={t} />}
    >
      <div className={hudStyles.mapModeSwitch} role="group" aria-label="Map view">
        <button type="button" aria-pressed={mapMode === "2d"} onClick={() => { setVisualMove(null); setOpenedBarrel(null); setMapMode("2d"); }}>2D</button>
        <button type="button" aria-pressed={mapMode === "3d"} disabled={!webglAvailable} onClick={() => setMapMode("3d")}>3D</button>
      </div>
      {mapMode === "3d" ? <HexMap3D snapshot={{ instanceId: snapshot.instanceId, cells: snapshot.cells, edges: snapshot.edges, actors: snapshot.actors, player: snapshot.player, questArtifact: snapshot.questArtifact }} visualMove={visualMove} openedBarrel={openedBarrel} connection={connection} visualNotices={visualNotices} selected={view.finished ? null : view.cell?.position ?? null} route={view.finished ? null : view.route ?? null} selectedTargetId={view.finished || !view.target ? null : targetId} selectedTargetKind={targetKind} indicators={indicators} busy={busy || !myTurn} labels={t}
        onCellClick={position => void handleCellClick(position)} onMonsterClick={actor => void selectMonster(actor)} onUnavailable={() => { setWebglAvailable(false); setMapMode("2d"); setError("WebGL is unavailable. Switched to 2D map."); }} />
        : <HexMap snapshot={{ cells: snapshot.cells, edges: snapshot.edges, actors: snapshot.actors, player: snapshot.player }} selected={view.finished ? null : view.cell?.position ?? null} route={view.finished ? null : view.route ?? null} selectedTargetId={view.finished || !view.target ? null : targetId} selectedTargetKind={targetKind} indicators={indicators} busy={busy || !myTurn} labels={t}
          onCellClick={position => void handleCellClick(position)} onMonsterClick={actor => void selectMonster(actor)} />}
    </HexBattleShell>
    {blueprintKey && <button type="button" className={hudStyles.placementCancel} onClick={() => { setBlueprintKey(null); pendingBuild.current = null; }}>{t.close} · {blueprintKey}</button>}
    {!view.finished && view.remaining >= 1 && view.remaining <= 5 && <div key={`${snapshot.turnNumber}:${view.remaining}`} className={hudStyles.finalSecond} aria-live="assertive">{view.remaining}</div>}
    {(introVisible && quest) && <div key={questNotice ? `found:${questNotice.userId}:${questNotice.q}:${questNotice.r}` : `intro:${instanceId}`} className={hudStyles.artifactFlight} role="status">
      <div className={hudStyles.artifactFlightHead}>{quest?.image && <img src={quest.image} alt="" />}<span><small>{t.questArtifact}</small><strong>{quest?.name || t.questArtifact}</strong></span></div>
      <p>{questNotice ? formatHexMessage(t.questAnnouncement, { artifact: quest?.name || t.questArtifact, player: questNotice.playerName || `${t.player} ${questNotice.userId}`, q: questNotice.q, r: questNotice.r }) : t.questGoal}</p>
    </div>}
    <HexActionNotices notices={notices} />
    {inventoryOpen && <Inventory pocketCapacity={snapshot.player?.pocketCapacity} pocketsUsed={snapshot.player?.pocketsUsed} craftStatus={snapshot.player?.consumableStatus} hexTargets={snapshot.actors.flatMap(actor => actor.kind === "player" && actor.userId !== undefined && actor.userId !== snapshot.player?.userId && (actor.health ?? 0) > 0 && snapshot.player && hexDistance(actor.position, snapshot.player.position) === 1 ? [{ id: actor.userId, name: actor.name || String(actor.userId), ally: actor.teamId === snapshot.player.teamId }] : [])} className={hudStyles.inventorySheet} onRequestClose={() => setInventoryOpen(false)} currentAP={snapshot.player?.ap ?? 0} personalTurn={snapshot.player?.personalTurn ?? 0} waterAvailableTurn={snapshot.player?.waterAvailableTurn ?? 0} maxAP={snapshot.player?.maxAP ?? 6} hexInventory={snapshot.player?.inventory ?? {}} equipmentLoot={snapshot.player?.equipmentLoot ?? []} questArtifactId={snapshot.questArtifactId} onHexUseItem={useItem} onHexBlueprintPlacementStart={key => { setBlueprintKey(key); setInventoryOpen(false); setRoute(null); }} hexBusy={busy || !myTurn} hexLabels={t} />}
    {(defeated || extracted) && <HexMatchResults snapshot={snapshot} labels={t} />}
  </div>;
}
