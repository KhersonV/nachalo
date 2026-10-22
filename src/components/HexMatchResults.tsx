"use client";

import type { HexSnapshot, HexResultItem } from "@/types/HexGameTypes";
import type { HexGameMessages } from "@/i18n/hexGame";
import GameResultShell from "./GameResultShell";
import styles from "@/styles/GameStatsPage.module.css";
import hexStyles from "@/styles/HexGameController.module.css";
import hudStyles from "@/features/hexHud/HexBattleShell.module.css";
import { resourceImage } from "@/utils/resourceAssets";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";

type Loot = { item_count?: number; item_name?: string; name?: string; image?: string };

function LootList({ items, empty, resources = false }: { items: Array<{ key: string; item: Loot }>; empty: string; resources?: boolean }) {
  const { language } = useGameLanguage();
  if (!items.length) return <p className={styles.rewardsEmpty}>{empty}</p>;
  return <div className={styles.lootGrid}>{items.map(({ key, item }) => {
    const image = resources && key.startsWith("resource_") ? resourceImage(item.item_name ?? item.name, item.image) ?? item.image : item.image;
    return <div className={styles.lootCard} key={key}>
    <div className={styles.lootImageFrame}>{image ? <img className={styles.lootImage} src={image} alt="" /> : <span className={styles.lootImageFallback}>◆</span>}</div>
    <div className={styles.lootBody}><strong>{uiText(language, item.name || item.item_name || key)}</strong> ×{item.item_count ?? 1}</div>
  </div>; })}</div>;
}

function EquipmentList({ items, fallback, empty }: { items?: HexResultItem[]; fallback: string[]; empty: string }) {
  return <LootList empty={empty} items={(items?.length ? items : fallback.map(name => ({ name }))).map((item, index) => ({ key: `${item.name}-${index}`, item }))} />;
}

export default function HexMatchResults({ snapshot, labels: t }: { snapshot: HexSnapshot; labels: HexGameMessages }) {
  const result = snapshot.result;
  const dead = result?.method === "defeat" || (!result && snapshot.player?.health === 0);
  const lostView = dead || result?.method === "opponent_extracted";
  const defeated = dead || (!result?.isWinner && result?.method === "opponent_extracted");
  const method = result?.method === "artifact" ? t.extractedArtifact : result?.method === "paid" ? t.extractedPaid : result?.method === "team_victory" ? t.teamVictory : result?.isWinner || result?.method === "victory" ? t.victory : result?.method === "opponent_extracted" ? t.opponentExtracted : t.defeatMessage;
  const loot = Object.entries(lostView ? result?.lost ?? {} : result?.transferred ?? {}).map(([key, item]) => ({ key, item }));
  return <div className={`${hexStyles.fullResultBackdrop} ${hudStyles.resultBackdrop}`} role="dialog" aria-modal="true" aria-label={t.resultSummary}>
    <GameResultShell><section className={`${styles.card} ${hudStyles.resultCard}`}>
      <header className={styles.resultHeader}>
        <p className={styles.eyebrow}>{t.resultSummary}</p>
        <h1 className={`${styles.title} ${defeated ? styles.titleLoss : styles.titleWin}`}>{defeated ? t.defeat : result?.isWinner || result?.method === "artifact" || result?.method === "victory" || result?.method === "team_victory" ? t.victory : t.extraction}</h1>
        <p className={styles.subtitle}>{method}</p>
		{result?.winnerUserId && <p className={styles.subtitle}>{t.winner}: {result.winnerUserId} · {t.team} {result.winnerTeamId}</p>}
		{result?.reason && <p className={styles.subtitle}>{t.reason}: {result.reason === "lightning" ? t.lightningDefeat : result.reason === "quest_artifact_extracted_by_opponent" ? t.opponentExtracted : result.reason === "teammate_extracted_quest_artifact" ? t.teamVictory : t.extractedArtifact}</p>}
      </header>
      <div className={styles.summaryGrid}>
        {snapshot.mode && <div className={styles.summaryItem}><span className={styles.summaryLabel}>{t.resultMode}</span><strong className={styles.summaryValue}>{snapshot.mode}</strong></div>}
        {result?.turnNumber != null && <div className={styles.summaryItem}><span className={styles.summaryLabel}>{t.resultTurn}</span><strong className={styles.summaryValue}>{result.turnNumber}</strong></div>}
        {result && <><div className={styles.summaryItem}><span className={styles.summaryLabel}>{t.goldEarned}</span><strong className={styles.summaryValue}>{result.goldEarned}</strong></div>
          <div className={styles.summaryItem}><span className={styles.summaryLabel}>{t.goldSpent}</span><strong className={styles.summaryValue}>{result.goldSpent}</strong></div>
          <div className={styles.summaryItem}><span className={styles.summaryLabel}>{t.xpEarned}</span><strong className={styles.summaryValue}>{result.xp}</strong></div></>}
        {result?.stats && <><div className={styles.summaryItem}><span className={styles.summaryLabel}>{t.monsterKills}</span><strong className={styles.summaryValue}>{result.stats.monsterKills}</strong></div>
          <div className={styles.summaryItem}><span className={styles.summaryLabel}>{t.playerKills}</span><strong className={styles.summaryValue}>{result.stats.playerKills}</strong></div>
          <div className={styles.summaryItem}><span className={styles.summaryLabel}>{t.damageDealt}</span><strong className={styles.summaryValue}>{result.stats.damageTotal}</strong></div>
          <div className={styles.summaryItem}><span className={styles.summaryLabel}>{t.damageTaken}</span><strong className={styles.summaryValue}>{result.stats.damageTaken}</strong></div></>}
      </div>
      {result && <><section className={styles.lootSection}><h2 className={styles.sectionTitle}>{lostView ? t.lostItems : t.movedStorage}</h2><LootList items={loot} empty={t.nothing} resources /></section>
        <section className={styles.lootSection}><h2 className={styles.sectionTitle}>{lostView ? t.equipmentLost : t.equipmentMoved}</h2><EquipmentList items={lostView ? result.lostEquipmentItems : result.equipmentItems} fallback={lostView ? result.lostEquipment ?? [] : result.equipment ?? []} empty={t.nothing} /></section>
        {result.wornEquipment && <section className={styles.lootSection}><h2 className={styles.sectionTitle}>{t.equipmentWorn}</h2><EquipmentList items={result.wornEquipment} fallback={[]} empty={t.nothing} /></section>}</>}
      <nav className={styles.actions}><a className={`${styles.button} ${styles.buttonPrimary}`} href={`/mode?repeatMode=${encodeURIComponent(snapshot.mode || "PVE")}`}>{t.playAgain}</a><a className={`${styles.button} ${styles.buttonSecondary}`} href="/base">{t.returnMenu}</a></nav>
    </section></GameResultShell>
  </div>;
}
