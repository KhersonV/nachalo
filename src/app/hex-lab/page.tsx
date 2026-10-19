"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { axialToWorld, HEX_DIRECTIONS, hexKey, hexNeighbor, type HexCoord } from "@/utils/hexGrid";
import { parseGeometry, parsePosition } from "@/types/geometry";
import { API_BASE, WS_URL } from "@/utils/serviceUrls";
import { useAuth } from "@/contexts/AuthContext";

type HexResource = { id: number; type: string; description: string; image: string };
type HexCell = { cellId: number; position: HexCoord; terrain: "floor" | "rock"; room?: number; resource?: HexResource };
type HexEdge = { position: HexCoord; direction: number; kind: "wall" };
type HexActor = { id: string; kind: "player" | "monster"; position: HexCoord; health?: number; maxHealth?: number };
type HexPlayer = { userId: number; position: HexCoord; energy: number; maxEnergy: number; health: number; mobility: number; inventory?: Record<string, { item_count?: number; item_name?: string }> };
type HexRoute = { path: HexCoord[]; stepCost: number; totalCost: number; energy: number; affordable: boolean; position: HexCoord };
type HexAttackPreview = { targetId: number; distance: number; cost: number; mode: "melee" | "ranged"; energy: number; targetHP: number };
type HexAttackResult = HexAttackPreview & { damage: number; counterDamage: number; newTargetHP: number; newAttackerHP: number; monsterDied: boolean; effects: Array<{ kind: string; amount?: number }>; drops: Array<{ name: string }>; stateRevision: number; mapRevision: number };
type HexPreview = {
  schemaVersion: 2;
  geometry: { kind: "hex"; coordinates: "axial"; orientation: "pointy"; version: 1 };
  instanceId: string;
  mapRevision: number;
  stateRevision: number;
  seed: number;
  cells: HexCell[];
  edges: HexEdge[];
  actors: HexActor[];
  player?: HexPlayer;
  activeUser?: number;
  turnNumber?: number;
};

const RADIUS = 22;
const PAD = 30;
const center = (position: HexCoord) => {
  const world = axialToWorld(position, RADIUS);
  return { x: world.x, y: world.z };
};
const vertices = (position: HexCoord) => {
  const c = center(position);
  return Array.from({ length: 6 }, (_, i) => {
    const angle = (-90 + i * 60) * Math.PI / 180;
    return `${c.x + RADIUS * Math.cos(angle)},${c.y + RADIUS * Math.sin(angle)}`;
  }).join(" ");
};
const wallEnds = (edge: HexEdge) => {
  const a = center(edge.position);
  const d = HEX_DIRECTIONS[edge.direction];
  const b = center({ q: edge.position.q + d.q, r: edge.position.r + d.r });
  const dx = b.x - a.x, dy = b.y - a.y;
  const length = Math.hypot(dx, dy);
  const mx = (a.x + b.x) / 2, my = (a.y + b.y) / 2;
  const tx = -dy / length * RADIUS / 2, ty = dx / length * RADIUS / 2;
  return { x1: mx - tx, y1: my - ty, x2: mx + tx, y2: my + ty };
};

function isPreview(value: unknown): value is HexPreview {
  if (!value || typeof value !== "object") return false;
  const data = value as Partial<HexPreview>;
  try {
    if (parseGeometry(data.geometry, data.schemaVersion).kind !== "hex") return false;
    if (!Array.isArray(data.cells) || !Array.isArray(data.edges) || !Array.isArray(data.actors)) return false;
    if (!data.cells.every(c => c && Number.isSafeInteger(c.cellId) && (c.terrain === "floor" || c.terrain === "rock") && !!parsePosition(c.position, data.geometry!))) return false;
    if (!data.edges.every(e => e && e.kind === "wall" && Number.isInteger(e.direction) && e.direction >= 0 && e.direction < 3 && !!parsePosition(e.position, data.geometry!))) return false;
    if (!data.actors.every(a => a && (a.kind === "player" || a.kind === "monster") && !!parsePosition(a.position, data.geometry!))) return false;
    if (data.player && (!Number.isSafeInteger(data.player.userId) || !Number.isSafeInteger(data.player.energy) || !parsePosition(data.player.position, data.geometry!))) return false;
    return true;
  } catch { return false; }
}

export default function HexLabPage() {
  const { user, isLoading } = useAuth();
  const [seedText, setSeedText] = useState("1");
  const [seed, setSeed] = useState(1);
  const [preview, setPreview] = useState<HexPreview | null>(null);
  const [liveInstanceId, setLiveInstanceId] = useState("");
  const [route, setRoute] = useState<HexRoute | null>(null);
  const [busy, setBusy] = useState(false);
  const pendingMove = useRef<{ destination: string; commandId: string } | null>(null);
  const pendingCollect = useRef<{ destination: string; commandId: string } | null>(null);
  const pendingTurn = useRef<{ turn: number; commandId: string } | null>(null);
  const pendingAttack = useRef<{ targetId: number; commandId: string } | null>(null);
  const [selectedTargetId, setSelectedTargetId] = useState<number | null>(null);
  const [attackPreview, setAttackPreview] = useState<HexAttackPreview | null>(null);
  const [attackOptions, setAttackOptions] = useState<Record<number, HexAttackPreview>>({});
  const routeRequest = useRef(0);
  const [selected, setSelected] = useState<HexCoord | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  useEffect(() => {
    if (typeof window !== "undefined") setLiveInstanceId(new URLSearchParams(window.location.search).get("match") ?? "");
  }, []);
  const loadLive = useCallback(async (id: string, token: string) => {
    const response = await fetch(`${API_BASE}/game/hex-test/${encodeURIComponent(id)}`, { headers: { Authorization: `Bearer ${token}` }, cache: "no-store" });
    if (!response.ok) throw new Error(await response.text());
    const data: unknown = await response.json();
    if (!isPreview(data) || !data.player) throw new Error("Некорректный HEX snapshot");
    setPreview(current => current?.instanceId === data.instanceId && current.stateRevision > data.stateRevision ? current : data);
    return data;
  }, []);
  useEffect(() => {
    if (!liveInstanceId || !user?.token) return;
    loadLive(liveInstanceId, user.token).then(() => { setError(""); setRoute(null); }).catch(reason => setError(String(reason)));
  }, [liveInstanceId, user?.token, loadLive]);
  useEffect(() => {
    if (!liveInstanceId || !user?.token || !preview?.player) { setAttackOptions({}); return; }
    let cancelled = false;
    const monsters = preview.actors.filter(actor => actor.kind === "monster");
    void Promise.all(monsters.map(async actor => {
      const targetId = Number(actor.id.replace(/^monster-/, ""));
      if (!Number.isSafeInteger(targetId)) return null;
      const response = await fetch(`${API_BASE}/game/hex-test/${encodeURIComponent(liveInstanceId)}/attack-preview`, { method: "POST", headers: { Authorization: `Bearer ${user.token}`, "Content-Type": "application/json" }, body: JSON.stringify({ targetId }) });
      return response.ok ? await response.json() as HexAttackPreview : null;
    })).then(options => { if (!cancelled) setAttackOptions(Object.fromEntries(options.filter((value): value is HexAttackPreview => !!value).map(value => [value.targetId, value]))); }).catch(() => { if (!cancelled) setAttackOptions({}); });
    return () => { cancelled = true; };
  }, [liveInstanceId, user?.token, preview?.stateRevision, preview?.instanceId]);
  useEffect(() => {
    if (!liveInstanceId || !user?.token) return;
    let closed = false;
    let socket: WebSocket | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    const connect = () => {
      const url = new URL(WS_URL);
      url.searchParams.set("token", user.token);
      url.searchParams.set("instanceId", liveInstanceId);
      socket = new WebSocket(url.toString());
      socket.onopen = () => { loadLive(liveInstanceId, user.token).catch(reason => setError(String(reason))); };
      socket.onmessage = event => {
        try {
          const message = JSON.parse(event.data);
          const payload = message?.payload;
          if (message?.type === "MATCH_PATCH" && payload?.instanceId === liveInstanceId && payload?.schemaVersion === 2) {
            // A snapshot handles duplicates, gaps and out-of-order delivery.
            loadLive(liveInstanceId, user.token).then(() => setRoute(null)).catch(reason => setError(String(reason)));
          }
        } catch { /* Other protocol messages belong to the square client. */ }
      };
      socket.onclose = () => { if (!closed) reconnectTimer = setTimeout(connect, 1500); };
    };
    connect();
    return () => { closed = true; if (reconnectTimer) clearTimeout(reconnectTimer); socket?.close(); };
  }, [liveInstanceId, user?.token, loadLive]);
  useEffect(() => {
    if (liveInstanceId) return;
    const controller = new AbortController();
    fetch(`${API_BASE}/game/hex-preview?seed=${seed}`, { signal: controller.signal })
      .then(async response => {
        if (!response.ok) throw new Error(`HEX API: ${response.status}. Проверьте HEX_PREVIEW_ENABLED=true и GameService.`);
        const data: unknown = await response.json();
        if (!isPreview(data)) throw new Error("Сервер вернул несовместимый HEX snapshot");
        setPreview(data); setSelected(null); setError("");
      })
      .catch(reason => { if (!controller.signal.aborted) setError(String(reason)); });
    return () => controller.abort();
  }, [seed, liveInstanceId]);

  const createLive = async () => {
    if (!user?.token) { setError("Войдите в аккаунт, чтобы создать тестовый матч."); return; }
    setBusy(true); setError("");
    try {
      const response = await fetch(`${API_BASE}/game/hex-test`, { method: "POST", headers: { Authorization: `Bearer ${user.token}`, "Content-Type": "application/json" }, body: JSON.stringify({ seed }) });
      if (!response.ok) throw new Error(await response.text());
      const data: unknown = await response.json();
      if (!isPreview(data) || !data.player) throw new Error("Некорректный HEX snapshot");
      setPreview(data); setSelected(null); setRoute(null); setLiveInstanceId(data.instanceId);
      window.history.replaceState(null, "", `/hex-lab?match=${encodeURIComponent(data.instanceId)}`);
    } catch (reason) { setError(String(reason)); } finally { setBusy(false); }
  };

  const selectCell = async (position: HexCoord) => {
    setSelectedTargetId(null); setAttackPreview(null);
    if (!liveInstanceId || !user?.token) { setSelected(position); return; }
    const key = hexKey(position);
    if (selected && hexKey(selected) === key && route && route.path.length > 0 && hexKey(route.path[route.path.length - 1]) === key) {
      if (!route.affordable) { setError(`Недостаточно энергии: нужно ${route.totalCost}, доступно ${route.energy}.`); return; }
      setBusy(true); setError("");
      if (!pendingMove.current || pendingMove.current.destination !== key) pendingMove.current = { destination: key, commandId: crypto.randomUUID() };
      try {
        const response = await fetch(`${API_BASE}/game/hex-test/${encodeURIComponent(liveInstanceId)}/move`, { method: "POST", headers: { Authorization: `Bearer ${user.token}`, "Content-Type": "application/json" }, body: JSON.stringify({ destination: position, commandId: pendingMove.current.commandId }) });
        if (!response.ok) throw new Error(await response.text());
        await loadLive(liveInstanceId, user.token);
        pendingMove.current = null; setSelected(null); setRoute(null);
      } catch (reason) { setError(String(reason)); } finally { setBusy(false); }
      return;
    }
    const requestID = ++routeRequest.current;
    setSelected(position); setRoute(null); setError(""); pendingMove.current = null;
    try {
      const response = await fetch(`${API_BASE}/game/hex-test/${encodeURIComponent(liveInstanceId)}/route`, { method: "POST", headers: { Authorization: `Bearer ${user.token}`, "Content-Type": "application/json" }, body: JSON.stringify({ destination: position }) });
      if (!response.ok) throw new Error(await response.text());
      const data = await response.json() as HexRoute;
      if (requestID !== routeRequest.current) return;
      setRoute(data);
      if (!data.affordable) setError(`Недостаточно энергии: нужно ${data.totalCost}, доступно ${data.energy}.`);
    } catch (reason) { if (requestID === routeRequest.current) setError(String(reason)); }
  };

  const selectMonster = async (actor: HexActor) => {
    if (!liveInstanceId || !user?.token || actor.kind !== "monster") return;
    const targetId = Number(actor.id.replace(/^monster-/, ""));
    if (!Number.isSafeInteger(targetId)) return;
    if (selectedTargetId === targetId && attackPreview?.targetId === targetId) {
      if (!pendingAttack.current || pendingAttack.current.targetId !== targetId) pendingAttack.current = { targetId, commandId: crypto.randomUUID() };
      setBusy(true); setError("");
      try {
        const response = await fetch(`${API_BASE}/game/hex-test/${encodeURIComponent(liveInstanceId)}/attack`, { method: "POST", headers: { Authorization: `Bearer ${user.token}`, "Content-Type": "application/json" }, body: JSON.stringify({ targetId, commandId: pendingAttack.current.commandId }) });
        if (!response.ok) throw new Error(await response.text());
        const result = await response.json() as HexAttackResult;
        await loadLive(liveInstanceId, user.token);
        const effects = result.effects?.map(effect => effect.kind).join(", ");
        setNotice(`Урон монстру ${result.damage}. Контратака ${result.counterDamage}. HP героя ${result.newAttackerHP}. ${effects ? `Эффекты: ${effects}. ` : ""}${result.monsterDied ? "Монстр погиб. " : ""}${result.drops?.length ? `Дроп: ${result.drops.map(drop => drop.name).join(", ")}.` : ""}`);
        pendingAttack.current = null; setSelectedTargetId(null); setAttackPreview(null);
      } catch (reason) { setError(String(reason)); } finally { setBusy(false); }
      return;
    }
    setSelected(null); setRoute(null); setSelectedTargetId(targetId); setAttackPreview(null); setError(""); pendingAttack.current = null;
    try {
      const response = await fetch(`${API_BASE}/game/hex-test/${encodeURIComponent(liveInstanceId)}/attack-preview`, { method: "POST", headers: { Authorization: `Bearer ${user.token}`, "Content-Type": "application/json" }, body: JSON.stringify({ targetId }) });
      if (!response.ok) throw new Error(await response.text());
      setAttackPreview(await response.json() as HexAttackPreview);
    } catch (reason) { setError(String(reason)); }
  };

  const collectResource = async () => {
    if (!liveInstanceId || !user?.token || !selectedCell?.resource) return;
    const key = hexKey(selectedCell.position);
    if (!pendingCollect.current || pendingCollect.current.destination !== key) pendingCollect.current = { destination: key, commandId: crypto.randomUUID() };
    setBusy(true); setError("");
    try {
      const response = await fetch(`${API_BASE}/game/hex-test/${encodeURIComponent(liveInstanceId)}/collect`, { method: "POST", headers: { Authorization: `Bearer ${user.token}`, "Content-Type": "application/json" }, body: JSON.stringify({ commandId: pendingCollect.current.commandId, position: selectedCell.position }) });
      if (!response.ok) throw new Error(await response.text());
      const result = await response.json() as { itemType: string };
      await loadLive(liveInstanceId, user.token);
      setNotice(`Подобрано: ${result.itemType}`);
      pendingCollect.current = null; setSelected(null); setRoute(null);
    } catch (reason) { setError(String(reason)); } finally { setBusy(false); }
  };

  const endTurn = async () => {
    if (!liveInstanceId || !user?.token || !preview?.turnNumber) return;
    const turn = preview.turnNumber;
    if (!pendingTurn.current || pendingTurn.current.turn !== turn) pendingTurn.current = { turn, commandId: crypto.randomUUID() };
    setBusy(true); setError("");
    try {
      const response = await fetch(`${API_BASE}/game/hex-test/${encodeURIComponent(liveInstanceId)}/end-turn`, { method: "POST", headers: { Authorization: `Bearer ${user.token}`, "Content-Type": "application/json" }, body: JSON.stringify({ commandId: pendingTurn.current.commandId, expectedTurn: turn }) });
      if (!response.ok) throw new Error(await response.text());
      const result = await response.json() as { turnNumber: number; energy: number };
      await loadLive(liveInstanceId, user.token);
      setNotice(`Ход ${result.turnNumber}. Энергия: ${result.energy}.`);
      pendingTurn.current = null; setRoute(null);
    } catch (reason) { setError(String(reason)); } finally { setBusy(false); }
  };

  const reachable = useMemo(() => {
    const result = new Set<string>();
    if (!preview?.player) return result;
    const floor = new Set(preview.cells.filter(cell => cell.terrain === "floor").map(cell => hexKey(cell.position)));
    const occupied = new Set(preview.actors.filter(actor => actor.kind === "monster").map(actor => hexKey(actor.position)));
    const walls = new Set(preview.edges.map(edge => {
      const next = hexNeighbor(edge.position, edge.direction);
      return [hexKey(edge.position), hexKey(next)].sort().join("|");
    }));
    const mobility = preview.player.mobility;
    const cost = mobility <= 2 ? 4 : mobility <= 5 ? 3 : mobility <= 8 ? 2 : 1;
    const maxSteps = Math.floor(preview.player.energy / cost);
    const queue: Array<{ position: HexCoord; steps: number }> = [{ position: preview.player.position, steps: 0 }];
    result.add(hexKey(preview.player.position));
    for (let head = 0; head < queue.length; head++) {
      const { position, steps } = queue[head];
      if (steps >= maxSteps) continue;
      for (let direction = 0; direction < 6; direction++) {
        const next = hexNeighbor(position, direction), nextKey = hexKey(next);
        if (!floor.has(nextKey) || occupied.has(nextKey) || result.has(nextKey) || walls.has([hexKey(position), nextKey].sort().join("|"))) continue;
        result.add(nextKey); queue.push({ position: next, steps: steps + 1 });
      }
    }
    result.delete(hexKey(preview.player.position));
    return result;
  }, [preview]);

  const bounds = useMemo(() => {
    if (!preview?.cells.length) return { x: -100, y: -100, width: 200, height: 200 };
    const points = preview.cells.map(cell => center(cell.position));
    const xs = points.map(p => p.x), ys = points.map(p => p.y);
    const x = Math.min(...xs) - RADIUS - PAD, y = Math.min(...ys) - RADIUS - PAD;
    return { x, y, width: Math.max(...xs) - x + RADIUS + PAD, height: Math.max(...ys) - y + RADIUS + PAD };
  }, [preview]);
  const cellByCoord = useMemo(() => new Map(preview?.cells.map(cell => [hexKey(cell.position), cell]) ?? []), [preview]);
  const selectedCell = selected ? cellByCoord.get(hexKey(selected)) : undefined;
  const download = () => {
    if (!preview) return;
    const url = URL.createObjectURL(new Blob([JSON.stringify(preview, null, 2)], { type: "application/json" }));
    const link = document.createElement("a");
    link.href = url; link.download = `nachalo-hex-v1-${preview.seed}.json`; link.click();
    URL.revokeObjectURL(url);
  };

  return <main style={{ minHeight: "100vh", background: "#101722", color: "#e6edf4", padding: 24, fontFamily: "sans-serif" }}>
    <h1>Nachalo · HEX Lab</h1>
    <p>Pointy-top карта. Тестовый матч использует ваш выбранный герой и сохраняется в PostgreSQL; квадратная игра работает отдельно.</p>
    {!liveInstanceId && <form onSubmit={event => { event.preventDefault(); const value = Number(seedText); if (Number.isSafeInteger(value)) setSeed(value); }} style={{ display: "flex", gap: 10, alignItems: "center", marginBottom: 16 }}>
      <label htmlFor="hex-seed">Seed</label><input id="hex-seed" value={seedText} onChange={event => setSeedText(event.target.value)} inputMode="numeric" style={{ width: 90, padding: 7 }} />
      <button type="submit">Создать карту</button>
    </form>}
    {!liveInstanceId && <button disabled={busy || isLoading || !user} onClick={createLive}>Создать игровой HEX-матч</button>}
    {liveInstanceId && <p>Матч: <code>{liveInstanceId}</code> · <button onClick={() => { setLiveInstanceId(""); setRoute(null); setSelected(null); window.history.replaceState(null, "", "/hex-lab"); }}>К генератору</button></p>}
    {error && <p role="alert" style={{ color: "#ff9c89" }}>{error}</p>}
    {notice && <p role="status" style={{ color: "#9ce5ae" }}>{notice}</p>}
    {preview && <>
      <p>hex-v1 · {preview.cells.length} клеток · {preview.edges.length} стен · mapRevision {preview.mapRevision} · stateRevision {preview.stateRevision} · <button onClick={download}>Скачать JSON</button></p>
      {preview.player && <div><p>Раунд {preview.turnNumber} · ход игрока {preview.activeUser} · HP {preview.player.health} · энергия {preview.player.energy}/{preview.player.maxEnergy} · позиция ({preview.player.position.q}, {preview.player.position.r})</p><p>Инвентарь матча: {Object.entries(preview.player.inventory ?? {}).map(([key, item]) => `${item.item_name ?? key} ×${item.item_count ?? 0}`).join(", ") || "пуст"}</p><button disabled={busy || preview.activeUser !== preview.player.userId} onClick={() => void endTurn()}>Завершить ход</button></div>}
      <svg role="img" aria-label="HEX карта подземелья" viewBox={`${bounds.x} ${bounds.y} ${bounds.width} ${bounds.height}`} style={{ display: "block", width: "min(100%, 960px)", background: "#080e16", border: "1px solid #3f5162", borderRadius: 12 }}>
        {preview.cells.map(cell => <polygon key={cell.cellId} points={vertices(cell.position)} onClick={() => { if (!busy) void selectCell(cell.position); }}
          fill={cell.terrain === "rock" ? "#202b37" : cell.room ? ["#445b66", "#496252", "#5b5268"][cell.room % 3] : "#394852"}
          stroke={selected && hexKey(selected) === hexKey(cell.position) ? "#ffe16a" : reachable.has(hexKey(cell.position)) ? "#50c9eb" : "#627383"}
          strokeWidth={selected && hexKey(selected) === hexKey(cell.position) ? 3 : reachable.has(hexKey(cell.position)) ? 2 : 0.8}
          style={{ cursor: "pointer" }}><title>{`(${cell.position.q}, ${cell.position.r}) · ${cell.terrain}`}</title></polygon>)}
        {route && preview.player && <polyline points={[preview.player.position, ...route.path].map(p => { const c = center(p); return `${c.x},${c.y}`; }).join(" ")} fill="none" stroke={route.affordable ? "#67f0f2" : "#ff987b"} strokeWidth={4} strokeDasharray={route.affordable ? undefined : "7 5"} pointerEvents="none" />}
        {preview.edges.map(edge => <line key={`${hexKey(edge.position)}:${edge.direction}`} {...wallEnds(edge)} stroke="#d8b87a" strokeWidth={4} strokeLinecap="round" pointerEvents="none" />)}
        {preview.cells.filter(cell => cell.resource).map(cell => { const p = center(cell.position); return <text key={`resource-${cell.cellId}`} x={p.x + 10} y={p.y - 8} pointerEvents="none" fontSize={15} fill="#f9d77b">◆</text>; })}
        {preview.actors.map(actor => { const p = center(actor.position); const targetId = Number(actor.id.replace(/^monster-/, "")); return <g key={actor.id} role={actor.kind === "monster" ? "button" : undefined} aria-label={actor.kind === "monster" ? `Монстр ${targetId}, HP ${actor.health ?? "?"}` : undefined} onClick={() => { if (!busy && actor.kind === "monster") void selectMonster(actor); }} style={{ cursor: actor.kind === "monster" ? "pointer" : "default" }}><circle cx={p.x} cy={p.y} r={actor.kind === "player" ? 11 : 9} fill={actor.kind === "player" ? "#57c7ed" : "#e87565"} stroke={selectedTargetId === targetId && actor.kind === "monster" ? "#ffe16a" : attackOptions[targetId] && actor.kind === "monster" ? "#7beba5" : "#fff"} strokeWidth={selectedTargetId === targetId && actor.kind === "monster" ? 4 : 2} /><text x={p.x} y={p.y+4} textAnchor="middle" fontSize={11} fill="#07131d" pointerEvents="none">{actor.kind === "player" ? "P" : "M"}</text>{actor.kind === "monster" && <text x={p.x} y={p.y+20} textAnchor="middle" fontSize={10} fill="#fff" stroke="#17202b" strokeWidth={2} paintOrder="stroke" pointerEvents="none">{actor.health}/{actor.maxHealth}</text>}<title>{`${actor.kind} · HP ${actor.health ?? "?"}/${actor.maxHealth ?? "?"}`}</title></g>; })}
      </svg>
      <p aria-live="polite">{selectedCell ? `Выбрана клетка (${selectedCell.position.q}, ${selectedCell.position.r}) · ${selectedCell.terrain} · room ${selectedCell.room ?? 0}` : "Выберите клетку для просмотра координат q/r"}{route && ` · маршрут ${route.path.length} шагов · стоимость ${route.totalCost} энергии${route.affordable ? " · второй клик подтверждает" : " · недостаточно энергии"}`}</p>
      {selectedCell?.resource && liveInstanceId && <p>Ресурс: {selectedCell.resource.type} · <button disabled={busy || preview.activeUser !== preview.player?.userId} onClick={() => void collectResource()}>Подобрать</button></p>}
      {selectedTargetId !== null && <p>Монстр {selectedTargetId} · HP {preview.actors.find(actor => actor.id === `monster-${selectedTargetId}`)?.health ?? "?"} · {attackPreview ? `дистанция ${attackPreview.distance}, ${attackPreview.mode === "melee" ? "ближняя" : "дальняя"} атака, стоимость ${attackPreview.cost} энергии. Второй клик по монстру подтверждает.` : "Атака пока недоступна или проверяется."} {attackPreview && <button disabled={busy} onClick={() => { const actor = preview.actors.find(value => value.id === `monster-${selectedTargetId}`); if (actor) void selectMonster(actor); }}>Атаковать</button>}</p>}
      <p style={{ color: "#9eb3c4" }}>Голубой маркер — персонаж, красные — монстры, золотые грани — стены, тёмные клетки — непроходимый камень.</p>
    </>}
  </main>;
}
