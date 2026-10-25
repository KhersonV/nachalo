"use client";

import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";
import { useId } from "react";
import type { SmartAction } from "./smartAction";
import styles from "./SmartActionButton.module.css";

function ActionIcon({ kind, icon }: { kind: SmartAction["kind"]; icon?: SmartAction["icon"] }) {
  return <svg viewBox="0 0 24 24" width="27" height="27" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    {icon === "herbs" || icon === "forest" ? <path d="M12 22V9m0 8C2 17 2 6 2 6s10 0 10 11Zm0-4c10 0 10-11 10-11s-10 0-10 11Z" /> : icon === "remains" ? <path d="M5 6c-4-5-8 1-3 4l12 9c-1 6 7 6 6 1 6 0 5-7 0-6L8 6C8 1 2 2 5 6Z" /> : kind === "attack" || kind === "structure" ? <><path d="m4 3 4 1 11 13-3 3L4 7ZM20 3l-4 1-4 5m-3 4-4 5m-2-2 5 5m8-14 4-4M3 21l3-3m12 0 3 3m-6-4 6-2" /></>
      : kind === "barrel" ? <><path d="m3 9 9-4 9 4-9 4Zm0 0v10l9 3 9-3V9M12 13v9M3 9l-2-4 8-3 3 3 3-3 8 3-2 4" /></>
      : kind === "portal" ? <><path d="M14 3H5v18h9M10 12h12m-4-4 4 4-4 4M8 5v14" /></>
      : kind === "collect" ? <><path d="m5 21 9-12M5 4c6-3 12 1 15 7l-5-2-3-4-7-1Z" /></>
      : kind === "pickup" ? <><path d="M8 12V6a2 2 0 0 1 4 0v6m0-4a2 2 0 0 1 4 0v4m0-2a2 2 0 0 1 4 0v6c0 4-3 6-6 6h-2c-2 0-4-2-5-4l-3-5a2 2 0 0 1 3-2l3 3" /></>
      : <path d="m5 3 14 10-7 1-3 7L5 3Z" />}
  </svg>;
}
export default function SmartActionButton({ action, onAction }: { action: SmartAction; onAction: () => void }) {
  const id = useId();
  const { language } = useGameLanguage();
  const category = action.kind === "attack" || action.kind === "structure" ? "attack" : action.kind === "portal" ? "portal" : action.kind === "none" ? "none" : "collect";
  const cost = [action.cost.ap ? `${action.cost.ap / 2} ОД` : "", action.cost.mp ? `${action.cost.mp / 2} ОП` : ""].filter(Boolean).join(" + ");
  const description = [action.label, cost && (action.icon ? `${uiText(language, "Cost")}: ${action.cost.ap / 2} AP` : `Стоимость: ${cost}`), action.distance !== undefined && `Дальность: ${action.distance} HEX`, action.hint, action.disabledReason].filter(Boolean).join("\n");
  return <div className={styles.control}>
    <button type="button" className={`${styles.button} ${styles[category]}`} aria-label={description} aria-describedby={id} aria-disabled={!action.enabled} aria-busy={action.pending} onClick={onAction}>
      {action.pending ? <span className={styles.spinner} aria-hidden="true" /> : <ActionIcon kind={action.kind} icon={action.icon} />}
    </button>
    <span id={id} role="tooltip" className={styles.tooltip}>{description}</span>
  </div>;
}
