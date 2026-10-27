"use client";

import { useGameLanguage } from "@/hooks/useGameLanguage";
import styles from "./HexLoading.module.css";
const text = {
  en: { preparing: "Preparing battle…", map: "Loading map…", connecting: "Connecting to battle…", characters: "Loading characters…", retry: "Retry", fallback: "Some visuals could not be loaded. The map remains available." },
  ru: { preparing: "Подготовка боя…", map: "Загрузка карты…", connecting: "Подключение к бою…", characters: "Загрузка персонажей…", retry: "Повторить", fallback: "Часть графики не загрузилась. Карта остаётся доступной." },
  de: { preparing: "Kampf wird vorbereitet…", map: "Karte wird geladen…", connecting: "Verbindung zum Kampf…", characters: "Charaktere werden geladen…", retry: "Erneut versuchen", fallback: "Einige Grafiken konnten nicht geladen werden. Die Karte bleibt verfügbar." },
};
export default function HexLoading({ stage = "preparing", local = false, error, onRetry }: {
  stage?: "preparing" | "map" | "connecting" | "characters"; local?: boolean; error?: string; onRetry?: () => void;
}) {
  const { language } = useGameLanguage();
  const t = text[language];
  return <div className={local ? styles.local : styles.screen} style={local ? { pointerEvents: "none" } : undefined} role={error ? "alert" : "status"} aria-live="polite">
    <div className={styles.card}>
      {!local && <strong className={styles.brand}>NACHALO</strong>}
      {!error && <span className={styles.spinner} aria-hidden="true" />}
      <p>{error === "visual" ? t.fallback : error || t[stage]}</p>
      {error && onRetry && <button type="button" onClick={onRetry}>{t.retry}</button>}
    </div>
  </div>;
}
