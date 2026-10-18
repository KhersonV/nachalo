"use client";

import { useMemo, useRef, useState } from "react";
import NachaloScene3D from "@/components/NachaloScene3D";
import type { RangerCue, RangerModel, RangerStatus } from "@/components/AnimatedRanger";
import { collectVisibleCellIndices, getCellIndex } from "@/utils/fogOfWar";
import { adaptRealMap, isWalkableTile, type RealMapSource } from "@/utils/realMapAdapter";
import fixture15 from "./fixtures/game-15.json";
import fixture15b from "./fixtures/game-15b.json";
import fixture24 from "./fixtures/game-24.json";

const MAPS = [
  { title: "GameService 15×15 A", data: fixture15 as unknown as RealMapSource },
  { title: "GameService 15×15 B", data: fixture15b as unknown as RealMapSource },
  { title: "GameService 24×24", data: fixture24 as unknown as RealMapSource },
];
const buttonStyle = { padding: "7px 12px", border: "1px solid #526277", borderRadius: 6,
  background: "#202e3e", color: "white", cursor: "pointer" } as const;

export default function GraphicsLabPage() {
  const [layout, setLayout] = useState(0);
  const [showTacticalGrid, setShowTacticalGrid] = useState(false);
  const [rangerCue, setRangerCue] = useState<RangerCue | null>(null);
  const [previewGeneration, setPreviewGeneration] = useState(0);
  const [rangerModel, setRangerModel] = useState<RangerModel>("animated");
  const [rangerStatus, setRangerStatus] = useState<RangerStatus | null>(null);
  const cueSequence = useRef(0);
  const [hero, setHero] = useState(() => {
    const [x, y] = MAPS[0].data.start_positions?.[0] ?? [1, 1];
    return { x, y };
  });
  const [explored, setExplored] = useState<Set<number>>(new Set());
  const source = MAPS[layout].data;
  const previewPlayers = useMemo(() => (source.start_positions ?? []).slice(1).map(([x, y], index) => ({
    user_id: index + 2, position: { x, y }, image: "/ranger/ranger.webp", name: `Старт ${index + 2}`,
  })), [source]);
  const previewSource = useMemo(() => ({ ...source, players: previewPlayers }), [source, previewPlayers]);
  const width = source.map_width, height = source.map_height;
  const visible = useMemo(() =>
    collectVisibleCellIndices([{ position: hero, sightRange: 3 }], width, height),
    [hero, width, height]);
  const allExplored = useMemo(() => new Set([...explored, ...visible]), [explored, visible]);
  const view = useMemo(() => adaptRealMap(previewSource, visible, allExplored),
    [previewSource, visible, allExplored]);

  const selectLayout = (next: number) => {
    const [x, y] = MAPS[next].data.start_positions?.[0] ?? [1, 1];
    setLayout(next);
    setHero({ x, y });
    setExplored(new Set());
  };

  const move = (dx: number, dy: number) => {
    const x = hero.x + dx, y = hero.y + dy;
    if (x < 0 || y < 0 || x >= width || y >= height) return;
    const tile = source.map.find(cell => cell.x === x && cell.y === y);
    if (!tile || !isWalkableTile(tile.tileCode)) return;
    setExplored(previous => new Set([...previous, ...visible]));
    setHero({ x, y });
  };
  const preview = (clip: RangerCue["clip"]) => {
    setRangerCue({ id: `lab:${++cueSequence.current}`, clip,
      target: clip === "Attack_Bow" ? { x: hero.x + 1, y: hero.y } : undefined });
  };

  return (
    <main style={{ minHeight: "100vh", background: "#101722", color: "white", padding: 20, fontFamily: "sans-serif" }}>
      <h1>Nachalo — Graphics Lab</h1>
      <p>Предпросмотр сохранённых карт, созданных GameService GenerateFullMap. Это фикстуры, не живые матчи. Значки других игроков обозначают стартовые позиции фикстуры.</p>
      <div style={{ display: "flex", gap: 8, marginBottom: 12, flexWrap: "wrap" }}>
        {MAPS.map((map, index) => <button key={map.title} onClick={() => selectLayout(index)}
          aria-pressed={layout === index}>{map.title}</button>)}
        <button onClick={() => setShowTacticalGrid(value => !value)}>
          Тактическая сетка: {showTacticalGrid ? "вкл" : "выкл"}
        </button>
      </div>
      <div style={{ display: "flex", gap: 8, marginBottom: 12, flexWrap: "wrap" }}>
        {(["animated", "legacy"] as const).map(model => <button key={model}
          aria-pressed={rangerModel === model}
          style={{ ...buttonStyle, background: rangerModel === model ? "#23545e" : buttonStyle.background,
            borderColor: rangerModel === model ? "#68ebff" : "#526277" }} onClick={() => {
            setRangerModel(model); setRangerCue(null); setRangerStatus(null);
          }}>{model === "animated" ? "Ranger Animated · 5 клипов" : "Ranger Legacy · 5 клипов"}</button>)}
      </div>
      <div style={{ display: "flex", gap: 8, marginBottom: 12, flexWrap: "wrap" }}>
        {([['Attack_Bow', 'выстрел'], ['Hit', 'урон'], ['Death', 'смерть']] as const).map(([clip, label]) =>
          <button key={clip}
            style={buttonStyle}
            onClick={() => preview(clip)}>Ranger: {label}</button>)}
        <button style={buttonStyle} onClick={() => { setRangerCue(null); setPreviewGeneration(value => value + 1); }}>Ranger: сброс / Idle</button>
      </div>
      {rangerModel === "animated" && <p style={{ color: "#a8b8ca" }}>
        Animated: Idle, Walk, Attack_Bow, Hit и Death. Выстрел выпускает стрелу через 1,417 с.
      </p>}
      <output aria-label="Состояние Ranger" data-clip={rangerStatus?.clip} data-clip-time={rangerStatus?.clipTime}
        data-arrow-visible={rangerStatus?.arrowVisible} style={{ display: "block", marginBottom: 12 }}>
        {!rangerStatus ? "Загрузка Ranger…" : rangerStatus.failed ? "Модель не загрузилась: показан резервный персонаж." :
          !rangerStatus.ready ? "Загрузка Ranger…" :
          `${rangerStatus.model === "animated" ? "Animated" : "Legacy"} · ${rangerStatus.clip} · ` +
          `${rangerStatus.speed.toFixed(2)} кл/с · Walk ×${rangerStatus.walkTimeScale.toFixed(2)} · ${rangerStatus.bones} костей`}
      </output>
      <NachaloScene3D key={layout} grid={source.map} preloadTerrain mapWidth={view.width} mapHeight={view.height}
        objects={view.objects} heroPosition={hero} visibleCells={visible} exploredCells={allExplored}
        ranger={{ position: hero, model: rangerModel, resetKey: previewGeneration,
          cues: rangerCue ? [rangerCue] : [] }} onRangerStatus={setRangerStatus}
        showTacticalGrid={showTacticalGrid} onMove={move}
        onCellClick={cell => {
          if (Math.abs(hero.x - cell.x) + Math.abs(hero.y - cell.y) === 1 &&
              view.walkable.has(getCellIndex(cell.x, cell.y, width))) move(cell.x - hero.x, cell.y - hero.y);
        }} />
      <div style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 12, flexWrap: "wrap" }}>
        <button onClick={() => move(0, -1)}>↑</button>
        <button onClick={() => move(-1, 0)}>←</button>
        <button onClick={() => move(0, 1)}>↓</button>
        <button onClick={() => move(1, 0)}>→</button>
        <span>Позиция: ({hero.x}, {hero.y}) · видимые: {visible.size} · разведано: {allExplored.size}</span>
      </div>
      <p style={{ color: "#a8b8ca" }}>Стрелки перемещают маркер только по проходимым клеткам. Колесо меняет масштаб. Тёмные клетки уже разведаны; неизвестные скрыты. Объекты показываются лишь при текущей видимости.</p>
    </main>
  );
}
