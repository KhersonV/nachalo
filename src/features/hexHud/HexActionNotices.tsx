"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import styles from "./HexActionNotices.module.css";
type Notice = { id: string; message: string; tone: "info" | "error" };
export function useActionNotices() {
  const [notices, setNotices] = useState<Notice[]>([]);
  const timers = useRef(new Map<string, number>());
  useEffect(() => () => timers.current.forEach(timer => window.clearTimeout(timer)), []);
  const notify = useCallback((message: string, tone: Notice["tone"] = "info") => {
    if (!message) return;
    const id = crypto.randomUUID();
    // Keep both the displayed queue and its timers bounded.
    if (timers.current.size >= 3) {
      const oldest = timers.current.keys().next().value;
      if (oldest) { window.clearTimeout(timers.current.get(oldest)); timers.current.delete(oldest); }
    }
    setNotices(current => [...current.slice(-2), { id, message, tone }]);
    timers.current.set(id, window.setTimeout(() => { timers.current.delete(id); setNotices(current => current.filter(item => item.id !== id)); }, 2800));
  }, []);
  return { notices, notify };
}
export default function HexActionNotices({ notices }: { notices: Notice[] }) {
  return <div className={styles.stack} aria-live="polite" aria-atomic="false">{notices.map(notice => <p key={notice.id} className={`${styles.notice} ${notice.tone === "error" ? styles.error : ""}`}>{notice.message}</p>)}</div>;
}
