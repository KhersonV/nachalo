"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { hexKey, type HexCoord } from "@/utils/hexGrid";
import { type HexActor, type HexAttackPreview, type HexAttackResult, type HexCombatIndicator, type HexPlayerResult, type HexRoute, type HexSnapshot } from "@/types/HexGameTypes";
import { hexAttackIndicators, hexMoveIndicators } from "@/utils/hexCombatIndicators";
import { useHexMatchSnapshot } from "@/hooks/useHexMatchSnapshot";
import type { ActionLogEntry, ActionLogCategory, ActionLogTone } from "@/types";
import HexMap from "./HexMap";
import PlayerHUD from "./PlayerHUD";
import Inventory from "./Inventory";
import ActionLog from "./ActionLog";
import HexMatchResults from "./HexMatchResults";
import styles from "@/styles/GameController.module.css";
import hexStyles from "@/styles/HexGameController.module.css";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { formatHexMessage, type GameLanguage } from "@/i18n/hexGame";

type Pending = { key: string; commandId: string };
type Props = { instanceId: string; initialSnapshot: HexSnapshot };
type HexBarrelResult = { outcome: "damage" | "resource" | "artifact"; damage?: number; health: number; itemName?: string };
type HexItemResult = { itemType: string; itemId: number; health: number; energy: number; axis?: "q" | "r"; value?: number };

export default function HexGameController({ instanceId, initialSnapshot }: Props) {
  const { user } = useAuth();
  const { language, setLanguage, messages: t } = useGameLanguage();
  const { snapshot, refresh, base, unavailable, syncError, questNotice, turnTimeoutNotice, visualNotices } = useHexMatchSnapshot(instanceId, initialSnapshot, user?.token);
  const [selected, setSelected] = useState<HexCoord | null>(null);
  const [route, setRoute] = useState<HexRoute | null>(null);
  const [targetId, setTargetId] = useState<number | null>(null);
	const [targetKind, setTargetKind] = useState<"player" | "monster">("monster");
	const [clockNow, setClockNow] = useState(Date.now());
  const [attackPreview, setAttackPreview] = useState<HexAttackPreview | null>(null);
  const [lastAttack, setLastAttack] = useState<HexAttackResult | null>(null);
  const [indicators, setIndicators] = useState<HexCombatIndicator[]>([]);
  const visualized = useRef(new Set<string>());
  const indicatorTimers = useRef<number[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [inventoryOpen, setInventoryOpen] = useState(false);
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
  useEffect(() => { if (unavailable || snapshot.finished || !snapshot.player || snapshot.player.health <= 0) { setTargetId(null); setAttackPreview(null); setLastAttack(null); setSelected(null); setRoute(null); } }, [unavailable, snapshot.finished, snapshot.player?.health]);
  useEffect(() => { setRoute(null); }, [snapshot.stateRevision]);
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
	const activeActor = snapshot.actors.find(actor => actor.userId === snapshot.activeUser);
	const remaining = snapshot.turnDeadline && !snapshot.finished ? Math.max(0, Math.ceil((Date.parse(snapshot.turnDeadline) - clockNow) / 1000)) : 0;
	const turnSeconds = snapshot.turnSeconds || 60;
  const contextAction = currentCell?.resource ? "collect" : currentCell?.barrel ? "open" : null;

  useEffect(() => {
    if (targetId === null) return;
    if (!target || !myTurn) {
      setTargetId(null); setAttackPreview(null); setLastAttack(null);
      return;
    }
    let cancelled = false;
    void post<HexAttackPreview>("attack-preview", { targetId, targetKind }).then(preview => {
      if (!cancelled) setAttackPreview(preview);
    }).catch(reason => {
      if (!cancelled) {
        setAttackPreview(null);
        if (!String(reason).includes("not enough energy")) { setTargetId(null); setLastAttack(null); }
        setError(String(reason));
      }
    });
    return () => { cancelled = true; };
  }, [targetId, targetKind, target, myTurn, snapshot.stateRevision, post]);

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
    const key = hexKey(position);
    if (snapshot.player && key === hexKey(snapshot.player.position)) { setSelected(position); setRoute(null); setError(""); return; }
    if (selected && hexKey(selected) === key && route?.path.length && hexKey(route.path[route.path.length - 1]) === key) {
      if (!route.affordable) { setError(formatHexMessage(t.noEnergy, { cost: route.totalCost, energy: route.energy })); return; }
      actionInFlight.current = true; setBusy(true); setError("");
      try {
        const moveCommandId = command(pendingMove, key);
        const moved = await post<HexRoute>("move", { destination: position, commandId: moveCommandId });
        showIndicators(moveCommandId, hexMoveIndicators(moved, moveCommandId));
        await refresh();
        pendingMove.current = null; setSelected(cellByCoord.get(key)?.barrel ? position : null); setRoute(null);
        record(`${t.moved} (${position.q}, ${position.r})`, "movement", "success");
      } catch (reason) { setError(String(reason)); } finally { actionInFlight.current = false; setBusy(false); }
      return;
    }
    const request = ++routeRequest.current;
    setSelected(position); setRoute(null); setError(""); pendingMove.current = null;
    try {
      const next = await post<HexRoute>("route", { destination: position });
      if (request !== routeRequest.current) return;
      setRoute(next);
      if (!next.affordable) setError(formatHexMessage(t.noEnergy, { cost: next.totalCost, energy: next.energy }));
    } catch (reason) { if (request === routeRequest.current) setError(String(reason)); }
  };
  const selectMonster = (actor: HexActor) => {
    if (busy || actionInFlight.current || !myTurn || actor.kind === "player" && actor.teamId === snapshot.player?.teamId) return;
    const id = actor.kind === "player" ? actor.userId ?? 0 : Number(actor.id.replace(/^monster-/, ""));
    if (!Number.isSafeInteger(id)) return;
    if (targetId === id && targetKind === actor.kind) return;
    setSelected(null); setRoute(null); setTargetId(id); setTargetKind(actor.kind); setAttackPreview(null); setLastAttack(null); setError(""); pendingAttack.current = null;
  };
  const attackTarget = async () => {
    if (busy || actionInFlight.current || !myTurn || targetId === null || !target || !attackPreview) return;
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

  return <div className={`${styles.container} ${hexStyles.hexGame}`} data-geometry="hex-v1">
    <div className={hexStyles.shell}>
      <aside className={hexStyles.leftHud}>
        {snapshot.player && <PlayerHUD health={snapshot.player.health} maxHealth={snapshot.player.maxHealth} energy={snapshot.player.energy}
          maxEnergy={snapshot.player.maxEnergy} agility={snapshot.player.agility} characterType={snapshot.player.characterType}
          isRanged={snapshot.player.isRanged} attackRange={snapshot.player.attackRange} labels={t} />}
        <ActionLog entries={log} compact labels={t} />
      </aside>
      <main className={hexStyles.stage}>
        <div className={hexStyles.mapContainer}>
          <HexMap snapshot={snapshot} selected={snapshot.finished || snapshot.player?.health === 0 ? null : selected} route={snapshot.finished || snapshot.player?.health === 0 ? null : route} selectedTargetId={snapshot.finished || snapshot.player?.health === 0 ? null : targetId} selectedTargetKind={targetKind} indicators={indicators} busy={busy || !myTurn} labels={t}
            onCellClick={position => void selectCell(position)} onMonsterClick={actor => void selectMonster(actor)} />
        </div>
      </main>
      <aside className={hexStyles.rightHud}>
        {quest && <section className={`${hexStyles.panel} ${hexStyles.questCard}`} aria-label={t.questArtifact}>
          <div className={hexStyles.questHeading}>{quest.image ? <img src={quest.image} alt={t.questImageAlt} /> : <span aria-hidden="true">◆</span>}<strong>{quest.name}</strong></div>
          <span>{questStatus}</span><small>{t.questGoal}</small>
        </section>}
        <div className={hexStyles.panel}>
          <div className={hexStyles.heading}><strong>HEX · {t.turn} {snapshot.turnNumber}</strong><label>{t.language} <select aria-label={t.language} value={language} onChange={event => setLanguage(event.target.value as GameLanguage)}><option value="en">EN</option><option value="ru">RU</option><option value="de">DE</option></select></label></div>
          <span>{snapshot.player?.name} · {snapshot.finished ? t.finished : snapshot.player && snapshot.player.health <= 0 ? t.defeatedWaiting : myTurn ? t.yourTurn : t.waiting}</span>
		  {!snapshot.finished && <><span>{t.activePlayer}: {snapshot.activePlayerName || activeActor?.name || `${t.player} ${snapshot.activeUser}`} · {remaining}s</span><progress max={turnSeconds} value={Math.min(turnSeconds, remaining)} aria-label={t.turnTime} /></>}
          {snapshot.actors.filter(actor => actor.kind === "player" && actor.userId !== snapshot.player?.userId).map(actor =>
            <span key={actor.id}>{actor.name || `${t.player} ${actor.userId}`} · {t.team} {actor.teamId} · HP {actor.health}/{actor.maxHealth}</span>)}
          <div className={hexStyles.actions}>
            {blueprintKey && <button onClick={() => { setBlueprintKey(null); pendingBuild.current = null; }}>{t.close} · {blueprintKey}</button>}
            {contextAction && <button className={hexStyles.contextAction} disabled={busy || !myTurn} onClick={() => contextAction === "collect" ? void collect() : void openBarrel().catch(() => {})}>{contextAction === "collect" ? t.pickUp : t.open}</button>}
            {atPortal && !snapshot.result && <button className={hexStyles.contextAction} disabled={busy || !myTurn || (!hasQuest && (snapshot.player?.balance ?? 0) < 50)} onClick={() => void extract()}>{hasQuest ? t.extractArtifact : t.extractPaid}</button>}
            {atPortal && !hasQuest && !snapshot.result && <span>{t.currentGold}: {snapshot.player?.balance ?? 0}{(snapshot.player?.balance ?? 0) < 50 ? ` · ${t.needGold}` : ""}</span>}
            <button onClick={() => setInventoryOpen(open => !open)}>{inventoryOpen ? t.closeInventory : t.inventory}</button>
            <button disabled={busy || !myTurn} onClick={() => void endTurn()}>{t.endTurn}</button>
            <button onClick={() => void refresh().catch(reason => setError(String(reason)))}>{t.refresh}</button>
          </div>
          <details className={hexStyles.debug}><summary>{t.debug}</summary>
            <span>{t.heroPosition}: ({snapshot.player?.position.q}, {snapshot.player?.position.r})</span>
            <span>mapRevision {snapshot.mapRevision} · stateRevision {snapshot.stateRevision}</span>
            {selectedCell && <span>{t.selectedCell}: ({selectedCell.position.q}, {selectedCell.position.r})</span>}
          </details>
        </div>
        {selectedCell && !snapshot.finished && <div className={hexStyles.panel}>{t.cell}: {selectedCell.terrain === "floor" ? t.floor : t.rock}
          {selectedCell.structureType && <span>{selectedCell.structureType} · HP {selectedCell.structureHealth}/30 · {t.team} {selectedCell.structureOwnerUserId}{selectedCell.isUnderConstruction ? ` · ${selectedCell.constructionTurnsLeft} ${t.turn}` : ""}</span>}
          {selectedCell.structureType && !selectedCell.isUnderConstruction && snapshot.player && (selectedCell.structureOwnerUserId !== snapshot.player.userId || selectedCell.structureType === "wall") && <button disabled={busy || !myTurn || snapshot.player.energy < 6} onClick={() => void attackStructure()}>{t.attack} · 6 {t.energy}</button>}
          {route && <span>{t.route}: {route.path.length} {t.steps} · {route.totalCost} {t.energy} · {route.affordable ? t.confirmRoute : t.insufficientEnergy}</span>}
          {selectedCell.resource && <><span>{t.resource}: {selectedCell.resource.type}</span>{hexKey(selectedCell.position) !== hexKey(snapshot.player?.position ?? { q: NaN, r: NaN }) && <span>{t.standOnResource}</span>}</>}
          {selectedCell.barrel && hexKey(selectedCell.position) !== hexKey(snapshot.player?.position ?? { q: NaN, r: NaN }) && <span>{t.barrel} · {t.standOnBarrel}</span>}
          {selectedCell.isPortal && <span>{snapshot.mode === "hex-test" ? t.portalUnavailable : hasQuest ? t.extractArtifact : t.extractPaid}</span>}
        </div>}
        {targetId !== null && target && !snapshot.finished && <div className={hexStyles.panel}>{targetKind === "player" ? t.player : t.monster} {target.name || targetId}
          <button onClick={() => { setTargetId(null); setAttackPreview(null); setLastAttack(null); }}>{t.close}</button>
          <span>HP {target.health}/{target.maxHealth}</span>
          {attackPreview ? <><span>{t.distance} {attackPreview.distance} · {attackPreview.mode} · {attackPreview.cost} {t.energy}</span>
            <button disabled={busy || !myTurn || attackPreview.energy < attackPreview.cost} onClick={() => void attackTarget()}>{t.attack}</button></> : <span>{t.attackChecking}</span>}
		  {lastAttack && <span>{t.lastHit}: {lastAttack.damage} {t.damage} · {t.counterattack} {lastAttack.counterDamage}{lastAttack.effects.length ? ` · ${t.effects}: ${lastAttack.effects.map(effect => effect.kind).join(", ")}` : ""}</span>}
        </div>}
      </aside>
    </div>
    {feedback && <p role="status" className={hexStyles.feedback}>{feedback}</p>}
    {questNotice && <p role="status" className={hexStyles.questNotice}>{formatHexMessage(t.questAnnouncement, { artifact: quest?.name || t.questArtifact, player: questNotice.playerName || `${t.player} ${questNotice.userId}`, q: questNotice.q, r: questNotice.r })}</p>}
	{turnTimeoutNotice && <p role="status" className={hexStyles.questNotice}>{t.turnTimedOut}</p>}
    {error && <p role="alert" className={hexStyles.error}>{error}</p>}
    {inventoryOpen && <Inventory hexInventory={snapshot.player?.inventory ?? {}} onHexUseItem={useItem} onHexBlueprintPlacementStart={key => { setBlueprintKey(key); setInventoryOpen(false); setRoute(null); }} hexBusy={busy || !myTurn} hexLabels={t} />}
    {(defeated || extracted) && <HexMatchResults snapshot={snapshot} labels={t} />}
  </div>;
}
