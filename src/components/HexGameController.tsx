"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { API_BASE, WS_URL } from "@/utils/serviceUrls";
import { hexKey, type HexCoord } from "@/utils/hexGrid";
import { isHexSnapshot, type HexActor, type HexAttackPreview, type HexAttackResult, type HexRoute, type HexSnapshot } from "@/types/HexGameTypes";
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
  const [snapshot, setSnapshot] = useState(initialSnapshot);
  const [selected, setSelected] = useState<HexCoord | null>(null);
  const [route, setRoute] = useState<HexRoute | null>(null);
  const [targetId, setTargetId] = useState<number | null>(null);
  const [attackPreview, setAttackPreview] = useState<HexAttackPreview | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [inventoryOpen, setInventoryOpen] = useState(false);
  const [log, setLog] = useState<ActionLogEntry[]>([]);
  const pendingMove = useRef<Pending | null>(null);
  const pendingAttack = useRef<Pending | null>(null);
  const pendingCollect = useRef<Pending | null>(null);
  const pendingTurn = useRef<Pending | null>(null);
  const routeRequest = useRef(0);
  const snapshotRevision = useRef(initialSnapshot.stateRevision);
  const base = `${API_BASE}/game/hex-test/${encodeURIComponent(instanceId)}`;

  const record = useCallback((message: string, category: ActionLogCategory, tone: ActionLogTone = "info") => {
    setLog(current => [...current.slice(-29), { id: crypto.randomUUID(), timestamp: Date.now(), message, category, tone }]);
  }, []);
  const refresh = useCallback(async () => {
    if (!user?.token) return;
    const response = await fetch(base, { headers: { Authorization: `Bearer ${user.token}` }, cache: "no-store" });
    if (!response.ok) throw new Error(`HEX snapshot ${response.status}: ${await response.text()}`);
    const next: unknown = await response.json();
    if (!isHexSnapshot(next) || !next.player || next.instanceId !== instanceId) throw new Error("Invalid HEX snapshot");
    if (next.stateRevision >= snapshotRevision.current) {
      snapshotRevision.current = next.stateRevision;
      setSnapshot(next);
    }
  }, [base, instanceId, user?.token]);
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

  useEffect(() => {
    if (!user?.token) return;
    let closed = false;
    let socket: WebSocket | null = null;
    let timer: ReturnType<typeof setTimeout> | null = null;
    const connect = () => {
      const url = new URL(WS_URL);
      url.searchParams.set("token", user.token);
      url.searchParams.set("instanceId", instanceId);
      socket = new WebSocket(url.toString());
      socket.onopen = () => {
        socket?.send(JSON.stringify({ type: "JOIN_MATCH", instanceId }));
        void refresh().catch(reason => setError(String(reason)));
      };
      socket.onmessage = event => {
        try {
          const message = JSON.parse(event.data);
          const payload = message?.payload;
          if (payload?.instanceId !== instanceId || payload?.schemaVersion !== 2) return;
          if (message.type === "MATCH_UPDATE" && isHexSnapshot(payload) && payload.player && payload.stateRevision >= snapshotRevision.current) {
            snapshotRevision.current = payload.stateRevision;
            setSnapshot(payload);
          } else if (message.type === "MATCH_PATCH" && payload.stateRevision > snapshotRevision.current) {
            setRoute(null);
            setAttackPreview(null);
            void refresh().catch(reason => setError(String(reason)));
          }
        } catch { /* Ignore unrelated socket frames. */ }
      };
      socket.onclose = () => { if (!closed) timer = setTimeout(connect, 1500); };
    };
    connect();
    return () => { closed = true; if (timer) clearTimeout(timer); socket?.close(); };
  }, [instanceId, refresh, user?.token]);

  const cellByCoord = useMemo(() => new Map(snapshot.cells.map(cell => [hexKey(cell.position), cell])), [snapshot.cells]);
  const selectedCell = selected ? cellByCoord.get(hexKey(selected)) : undefined;
  const myTurn = !!snapshot.player && snapshot.activeUser === snapshot.player.userId && snapshot.player.health > 0;

  const selectCell = async (position: HexCoord) => {
    if (busy || !myTurn) return;
    setTargetId(null); setAttackPreview(null);
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
  const selectMonster = async (actor: HexActor) => {
    if (busy || !myTurn || actor.kind !== "monster") return;
    const id = Number(actor.id.replace(/^monster-/, ""));
    if (!Number.isSafeInteger(id)) return;
    if (targetId === id && attackPreview?.targetId === id) {
      setBusy(true); setError("");
      try {
        const result = await post<HexAttackResult>("attack", { targetId: id, commandId: command(pendingAttack, String(id)) });
        await refresh();
        pendingAttack.current = null; setTargetId(null); setAttackPreview(null);
        record(`Монстр ${id}: урон ${result.damage}, контратака ${result.counterDamage}${result.monsterDied ? ", убит" : ""}`, "attack", "success");
      } catch (reason) { setError(String(reason)); } finally { setBusy(false); }
      return;
    }
    setSelected(null); setRoute(null); setTargetId(id); setAttackPreview(null); setError(""); pendingAttack.current = null;
    try { setAttackPreview(await post<HexAttackPreview>("attack-preview", { targetId: id })); }
    catch (reason) { setError(String(reason)); }
  };
  const collect = async () => {
    if (busy || !myTurn || !selectedCell?.resource) return;
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
    <div className={styles.matchShell}>
      <aside className={styles.leftHudColumn}>
        {snapshot.player && <PlayerHUD health={snapshot.player.health} maxHealth={snapshot.player.maxHealth} energy={snapshot.player.energy}
          maxEnergy={snapshot.player.maxEnergy} agility={snapshot.player.agility} characterType={snapshot.player.characterType}
          isRanged={snapshot.player.isRanged} attackRange={snapshot.player.attackRange} />}
        <ActionLog entries={log} />
      </aside>
      <main className={`${styles.mapStage} ${hexStyles.stage}`}>
        <div className={styles.mapContainer}>
          <HexMap snapshot={snapshot} selected={selected} route={route} selectedTargetId={targetId} busy={busy}
            onCellClick={position => void selectCell(position)} onMonsterClick={actor => void selectMonster(actor)} />
        </div>
      </main>
      <aside className={`${styles.rightHudColumn} ${hexStyles.info}`}>
        <div className={hexStyles.panel}>
          <strong>HEX · Ход {snapshot.turnNumber}</strong>
          <span>{snapshot.player?.name} · {myTurn ? "Ваш ход" : "Ожидание хода"}</span>
          <span>Позиция: ({snapshot.player?.position.q}, {snapshot.player?.position.r})</span>
          <span>Ресурсы: {Object.entries(snapshot.player?.inventory ?? {}).map(([key, item]) => `${item.item_name ?? key} ×${item.item_count ?? 0}`).join(", ") || "пусто"}</span>
          <span>mapRevision {snapshot.mapRevision} · stateRevision {snapshot.stateRevision}</span>
          <button onClick={() => setInventoryOpen(open => !open)}>{inventoryOpen ? "Закрыть инвентарь" : "Инвентарь"}</button>
          <button disabled={busy || !myTurn} onClick={() => void endTurn()}>Завершить ход</button>
          <button onClick={() => void refresh().catch(reason => setError(String(reason)))}>Обновить состояние</button>
        </div>
        {selectedCell && <div className={hexStyles.panel}>Клетка ({selectedCell.position.q}, {selectedCell.position.r}) · {selectedCell.terrain}
          {route && <span>Маршрут: {route.path.length} шагов · {route.totalCost} энергии · {route.affordable ? "второй клик подтверждает" : "не хватает энергии"}</span>}
          {selectedCell.resource && <><span>Ресурс: {selectedCell.resource.type}</span><button disabled={busy || !myTurn} onClick={() => void collect()}>Подобрать</button></>}
          {selectedCell.barrel && <span>Бочка · открытие пока недоступно</span>}
          {selectedCell.isPortal && <span>Портал · эвакуация пока недоступна</span>}
        </div>}
        {targetId !== null && <div className={hexStyles.panel}>Монстр {targetId}
          {attackPreview ? <><span>Дистанция {attackPreview.distance} · {attackPreview.mode} · {attackPreview.cost} энергии</span>
            <button disabled={busy || !myTurn} onClick={() => { const actor = snapshot.actors.find(a => a.id === `monster-${targetId}`); if (actor) void selectMonster(actor); }}>Атаковать</button></> : <span>Атака пока недоступна или проверяется.</span>}
        </div>}
        {error && <p role="alert" className={hexStyles.error}>{error}</p>}
      </aside>
    </div>
    {inventoryOpen && <Inventory hexInventory={snapshot.player?.inventory ?? {}} />}
  </div>;
}
