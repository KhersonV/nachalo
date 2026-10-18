"use client";

import { useMemo, useRef, useState } from "react";
import NachaloScene3D from "@/components/NachaloScene3D";
import type { RangerCue } from "@/components/AnimatedRanger";
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

export default function GraphicsLabPage() {
  const [layout, setLayout] = useState(0);
  const [showTacticalGrid, setShowTacticalGrid] = useState(false);
  const [rangerCue, setRangerCue] = useState<RangerCue | null>(null);
  const [previewGeneration, setPreviewGeneration] = useState(0);
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
        <button onClick={() => preview("Attack_Bow")}>Ranger: выстрел</button>
        <button onClick={() => preview("Hit")}>Ranger: урон</button>
        <button onClick={() => preview("Death")}>Ranger: смерть</button>
        <button onClick={() => { setRangerCue(null); setPreviewGeneration(value => value + 1); }}>Ranger: сброс / Idle</button>
      </div>
      <NachaloScene3D key={`${layout}:${previewGeneration}`} grid={view.grid} mapWidth={view.width} mapHeight={view.height}
        objects={view.objects} heroPosition={hero} visibleCells={visible} exploredCells={allExplored}
        ranger={{ position: hero, cues: rangerCue ? [rangerCue] : [] }}
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
