"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { hexKey, type HexCoord } from "@/utils/hexGrid";
import { type HexActor, type HexAttackPreview, type HexAttackResult, type HexRoute, type HexSnapshot } from "@/types/HexGameTypes";
import { useHexMatchSnapshot } from "@/hooks/useHexMatchSnapshot";
import type { ActionLogEntry, ActionLogCategory, ActionLogTone } from "@/types";
import HexMap from "./HexMap";
import PlayerHUD from "./PlayerHUD";
import Inventory from "./Inventory";
import ActionLog from "./ActionLog";
import styles from "@/styles/GameController.module.css";
import hexStyles from "@/styles/HexGameController.module.css";

type Pending = { key: string; commandId: string };
type Props = { instanceId: string; initialSnapshot: HexSnapshot };

export default function HexGameController({ instanceId, initialSnapshot }: Props) {
  const { user } = useAuth();
  const { snapshot, refresh, base, unavailable, syncError } = useHexMatchSnapshot(instanceId, initialSnapshot, user?.token);
  const [selected, setSelected] = useState<HexCoord | null>(null);
  const [route, setRoute] = useState<HexRoute | null>(null);
  const [targetId, setTargetId] = useState<number | null>(null);
  const [attackPreview, setAttackPreview] = useState<HexAttackPreview | null>(null);
  const [lastAttack, setLastAttack] = useState<HexAttackResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [inventoryOpen, setInventoryOpen] = useState(false);
  const [feedback, setFeedback] = useState("");
  const [log, setLog] = useState<ActionLogEntry[]>([]);
  const pendingMove = useRef<Pending | null>(null);
  const pendingAttack = useRef<Pending | null>(null);
  const pendingCollect = useRef<Pending | null>(null);
  const pendingTurn = useRef<Pending | null>(null);
  const routeRequest = useRef(0);

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
  useEffect(() => { if (unavailable) { setTargetId(null); setAttackPreview(null); setLastAttack(null); } }, [unavailable]);
  useEffect(() => { setRoute(null); }, [snapshot.stateRevision]);
  const post = useCallback(async <T,>(action: string, body: object): Promise<T> => {
    if (!user?.token) throw new Error("Session unavailable");
    const response = await fetch(`${base}/${action}`, {
      method: "POST", headers: { Authorization: `Bearer ${user.token}`, "Content-Type": "application/json" }, body: JSON.stringify(body),
    });
    if (!response.ok) throw new Error(await response.text());
    return response.json() as Promise<T>;
  }, [base, user?.token]);
  const command = useCallback((ref: { current: Pending | null }, key: string) => {
    if (!ref.current || ref.current.key !== key) ref.current = { key, commandId: crypto.randomUUID() };
    return ref.current.commandId;
  }, []);

  const cellByCoord = useMemo(() => new Map(snapshot.cells.map(cell => [hexKey(cell.position), cell])), [snapshot.cells]);
  const selectedCell = selected ? cellByCoord.get(hexKey(selected)) : undefined;
  const myTurn = !!snapshot.player && snapshot.activeUser === snapshot.player.userId && snapshot.player.health > 0;
  const target = targetId === null ? undefined : snapshot.actors.find(actor => actor.id === `monster-${targetId}`);
  const onResourceCell = !!selectedCell?.resource && !!snapshot.player && hexKey(selectedCell.position) === hexKey(snapshot.player.position);

  useEffect(() => {
    if (targetId === null) return;
    if (!target || !myTurn) {
      setTargetId(null); setAttackPreview(null); setLastAttack(null);
      return;
    }
    let cancelled = false;
    void post<HexAttackPreview>("attack-preview", { targetId }).then(preview => {
      if (!cancelled) setAttackPreview(preview);
    }).catch(reason => {
      if (!cancelled) {
        setAttackPreview(null);
        if (!String(reason).includes("not enough energy")) { setTargetId(null); setLastAttack(null); }
        setError(String(reason));
      }
    });
    return () => { cancelled = true; };
  }, [targetId, target, myTurn, snapshot.stateRevision, post]);

  const selectCell = async (position: HexCoord) => {
    if (busy || !myTurn) return;
    const occupant = snapshot.actors.find(actor => hexKey(actor.position) === hexKey(position));
    if (occupant?.kind === "monster") { selectMonster(occupant); return; }
    if (snapshot.actors.some(actor => actor.kind === "player" && actor.userId !== snapshot.player?.userId && hexKey(actor.position) === hexKey(position))) {
      setSelected(position); setRoute(null); setError("PvP-атаки пока недоступны в HEX."); return;
    }
    setTargetId(null); setAttackPreview(null); setLastAttack(null);
    const key = hexKey(position);
    if (selected && hexKey(selected) === key && route?.path.length && hexKey(route.path[route.path.length - 1]) === key) {
      if (!route.affordable) { setError(`Недостаточно энергии: нужно ${route.totalCost}, доступно ${route.energy}.`); return; }
      setBusy(true); setError("");
      try {
        await post("move", { destination: position, commandId: command(pendingMove, key) });
        await refresh();
        pendingMove.current = null; setSelected(null); setRoute(null);
        record(`Перемещение в (${position.q}, ${position.r})`, "movement", "success");
      } catch (reason) { setError(String(reason)); } finally { setBusy(false); }
      return;
    }
    const request = ++routeRequest.current;
    setSelected(position); setRoute(null); setError(""); pendingMove.current = null;
    try {
      const next = await post<HexRoute>("route", { destination: position });
      if (request !== routeRequest.current) return;
      setRoute(next);
      if (!next.affordable) setError(`Недостаточно энергии: нужно ${next.totalCost}, доступно ${next.energy}.`);
    } catch (reason) { if (request === routeRequest.current) setError(String(reason)); }
  };
  const selectMonster = (actor: HexActor) => {
    if (busy || !myTurn || actor.kind !== "monster") return;
    const id = Number(actor.id.replace(/^monster-/, ""));
    if (!Number.isSafeInteger(id)) return;
    if (targetId === id) return;
    setSelected(null); setRoute(null); setTargetId(id); setAttackPreview(null); setLastAttack(null); setError(""); pendingAttack.current = null;
  };
  const attackTarget = async () => {
    if (busy || !myTurn || targetId === null || !target || !attackPreview) return;
    const id = targetId;
    setBusy(true); setError("");
    try {
      const result = await post<HexAttackResult>("attack", { targetId: id, commandId: command(pendingAttack, String(id)) });
      pendingAttack.current = null;
      setLastAttack(result);
      if (result.monsterDied || result.newTargetHP <= 0 || result.newAttackerHP <= 0) {
        setTargetId(null); setAttackPreview(null);
      } else {
        setAttackPreview(previous => previous ? { ...previous, targetHP: result.newTargetHP, energy: result.energy } : previous);
      }
      await refresh();
      record(`Монстр ${id}: урон ${result.damage}, контратака ${result.counterDamage}${result.monsterDied ? ", убит" : ""}`, "attack", "success");
    } catch (reason) { setError(String(reason)); } finally { setBusy(false); }
  };
  const collect = async () => {
    if (busy || !myTurn || !onResourceCell || !selectedCell?.resource) return;
    setBusy(true); setError("");
    try {
      await post("collect", { position: selectedCell.position, commandId: command(pendingCollect, hexKey(selectedCell.position)) });
      await refresh();
      record(`Подобрано: ${selectedCell.resource.type}`, "resource", "success");
      pendingCollect.current = null; setSelected(null); setRoute(null);
    } catch (reason) { setError(String(reason)); } finally { setBusy(false); }
  };
  const endTurn = async () => {
    if (busy || !myTurn || !snapshot.turnNumber) return;
    setBusy(true); setError("");
    try {
      await post("end-turn", { expectedTurn: snapshot.turnNumber, commandId: command(pendingTurn, String(snapshot.turnNumber)) });
      await refresh();
      record(`Ход ${snapshot.turnNumber} завершён`, "turn", "success");
      pendingTurn.current = null; setSelected(null); setRoute(null);
    } catch (reason) { setError(String(reason)); } finally { setBusy(false); }
  };

  return <div className={`${styles.container} ${hexStyles.hexGame}`} data-geometry="hex-v1">
    <div className={hexStyles.shell}>
      <aside className={hexStyles.leftHud}>
        {snapshot.player && <PlayerHUD health={snapshot.player.health} maxHealth={snapshot.player.maxHealth} energy={snapshot.player.energy}
          maxEnergy={snapshot.player.maxEnergy} agility={snapshot.player.agility} characterType={snapshot.player.characterType}
          isRanged={snapshot.player.isRanged} attackRange={snapshot.player.attackRange} />}
        <ActionLog entries={log} compact />
      </aside>
      <main className={hexStyles.stage}>
        <div className={hexStyles.mapContainer}>
          <HexMap snapshot={snapshot} selected={selected} route={route} selectedTargetId={targetId} busy={busy}
            onCellClick={position => void selectCell(position)} onMonsterClick={actor => void selectMonster(actor)} />
        </div>
      </main>
      <aside className={hexStyles.rightHud}>
        <div className={hexStyles.panel}>
          <strong>HEX · Ход {snapshot.turnNumber}</strong>
          <span>{snapshot.player?.name} · {myTurn ? "Ваш ход" : "Ожидание хода"}</span>
          {snapshot.actors.filter(actor => actor.kind === "player" && actor.userId !== snapshot.player?.userId).map(actor =>
            <span key={actor.id}>{actor.name || `Игрок ${actor.userId}`} · команда {actor.teamId} · HP {actor.health}/{actor.maxHealth}</span>)}
          <span>Ресурсы: {Object.entries(snapshot.player?.inventory ?? {}).map(([key, item]) => `${item.item_name ?? key} ×${item.item_count ?? 0}`).join(", ") || "пусто"}</span>
          {snapshot.mode !== "PVE" && snapshot.mode !== "hex-test" && <span>PvP-атаки пока недоступны в HEX; доступны движение и завершение хода.</span>}
          <div className={hexStyles.actions}>
            <button onClick={() => setInventoryOpen(open => !open)}>{inventoryOpen ? "Закрыть инвентарь" : "Инвентарь"}</button>
            <button disabled={busy || !myTurn} onClick={() => void endTurn()}>Завершить ход</button>
            <button onClick={() => void refresh().catch(reason => setError(String(reason)))}>Обновить</button>
          </div>
          <details className={hexStyles.debug}><summary>Диагностика</summary>
            <span>Позиция героя: ({snapshot.player?.position.q}, {snapshot.player?.position.r})</span>
            <span>mapRevision {snapshot.mapRevision} · stateRevision {snapshot.stateRevision}</span>
            {selectedCell && <span>Выбрана клетка: ({selectedCell.position.q}, {selectedCell.position.r})</span>}
          </details>
        </div>
        {selectedCell && <div className={hexStyles.panel}>Клетка: {selectedCell.terrain === "floor" ? "каменный пол" : "препятствие"}
          {route && <span>Маршрут: {route.path.length} шагов · {route.totalCost} энергии · {route.affordable ? "второй клик подтверждает" : "не хватает энергии"}</span>}
          {selectedCell.resource && <><span>Ресурс: {selectedCell.resource.type}</span><button disabled={busy || !myTurn || !onResourceCell} onClick={() => void collect()}>Подобрать</button>{!onResourceCell && <span>Для подбора встаньте на эту клетку.</span>}</>}
          {selectedCell.barrel && <span>Бочка · для открытия нужно стоять на её клетке; открытие пока недоступно</span>}
          {selectedCell.isPortal && <span>Портал · эвакуация пока недоступна</span>}
        </div>}
        {targetId !== null && target && <div className={hexStyles.panel}>Монстр {targetId}
          <button onClick={() => { setTargetId(null); setAttackPreview(null); setLastAttack(null); }}>Закрыть</button>
          <span>HP {target.health}/{target.maxHealth}</span>
          {lastAttack && <span>Последний удар: {lastAttack.damage} урона · контратака {lastAttack.counterDamage}{lastAttack.effects.length ? ` · эффекты: ${lastAttack.effects.map(effect => effect.kind).join(", ")}` : ""}</span>}
          {attackPreview ? <><span>Дистанция {attackPreview.distance} · {attackPreview.mode} · {attackPreview.cost} энергии</span>
            <button disabled={busy || !myTurn || attackPreview.energy < attackPreview.cost} onClick={() => void attackTarget()}>Атаковать</button></> : <span>Атака пока недоступна или проверяется.</span>}
        </div>}
      </aside>
    </div>
    {feedback && <p role="status" className={hexStyles.feedback}>{feedback}</p>}
    {error && <p role="alert" className={hexStyles.error}>{error}</p>}
    {inventoryOpen && <Inventory hexInventory={snapshot.player?.inventory ?? {}} />}
  </div>;
}
