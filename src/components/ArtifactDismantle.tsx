"use client";

import React from "react";
import { useAuth } from "@/contexts/AuthContext";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";
import { API_BASE } from "@/utils/serviceUrls";
import { ConfirmDialog } from "./ui";

type Preview = { artifactId: number; name: string; quantity: number; owned: number; yields: { code: string; name: string; quantity: number }[] };
export default function ArtifactDismantle({ artifactId, onDone }: { artifactId: number; onDone: () => Promise<void> }) {
 const { user } = useAuth(); const { language } = useGameLanguage();
 const t = (s: string) => uiText(language, s);
 const [refreshVersion, setRefreshVersion] = React.useState(0);
 const [quantity, setQuantity] = React.useState(1);
 const [preview, setPreview] = React.useState<Preview | null>(null);
 const [error, setError] = React.useState(""); const [busy, setBusy] = React.useState(false);
 const [confirm, setConfirm] = React.useState(false); const [notice, setNotice] = React.useState("");
 const lock = React.useRef(false); const pending = React.useRef<{ key: string; commandId: string } | null>(null);
 React.useEffect(() => {
  const controller = new AbortController(); setPreview(null); setError(""); setConfirm(false);
  if (!user?.token) return () => controller.abort();
  void fetch(`${API_BASE}/game/artifacts/dismantle?artifactId=${artifactId}&quantity=${quantity}`, { headers: { Authorization: `Bearer ${user.token}` }, cache: "no-store", signal: controller.signal })
   .then(async response => { const data = await response.json(); if (!response.ok) throw new Error(data.error || "artifact_dismantle_unavailable"); if (!controller.signal.aborted) setPreview(data); })
   .catch(reason => { if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : String(reason)); });
  return () => controller.abort();
 }, [artifactId, quantity, user?.token, refreshVersion]);
 const dismantle = async () => {
  if (lock.current || !preview || !user?.token) return;
  lock.current = true; setBusy(true); setError("");
  const shown = preview; const key = `${shown.artifactId}:${shown.quantity}`;
  if (pending.current?.key !== key) pending.current = { key, commandId: crypto.randomUUID() };
  try {
   const response = await fetch(`${API_BASE}/game/artifacts/dismantle`, { method: "POST", headers: { Authorization: `Bearer ${user.token}`, "Content-Type": "application/json" }, body: JSON.stringify({ artifactId: shown.artifactId, quantity: shown.quantity, commandId: pending.current!.commandId }) });
   const result = await response.json();
   if (!response.ok) throw new Error(result.error || "artifact_dismantle_unavailable");
   pending.current = null; setConfirm(false); setPreview(null);
   setNotice(`${t("Materials received")}: ${result.yields.map((v: { name: string; quantity: number }) => `${t(v.name)} ×${v.quantity}`).join(", ")}`);
   await onDone(); setRefreshVersion(v => v + 1);
  } catch (reason) { setError(reason instanceof Error ? reason.message : String(reason)); }
  finally { lock.current = false; setBusy(false); }
 };
 return <section>
  <h4>{t("Dismantle artifact")}</h4>
  <label>{t("Quantity")} <input type="number" min={1} max={Math.min(100, preview?.owned ?? 100)} value={quantity} disabled={busy} onChange={e => setQuantity(Math.min(100, Math.max(1, Math.trunc(Number(e.target.value) || 1))))} /></label>
  {preview && <><p>{t(preview.name)} ×{preview.quantity} · {t("Available")}: {preview.owned}</p><ul>{preview.yields.map(v => <li key={v.code}>{t(v.name)} ×{v.quantity}</li>)}</ul><button type="button" disabled={busy} onClick={() => setConfirm(true)}>{t("Dismantle")}</button></>}
  {error && <p role="alert">{t(error)} <button type="button" disabled={busy} onClick={() => setRefreshVersion(v => v + 1)}>{t("Refresh")}</button></p>}{notice && <p role="status">{notice}</p>}
  <ConfirmDialog open={confirm && !!preview} title={t("Dismantle artifact?")} description={preview ? `${t(preview.name)} ×${preview.quantity} → ${preview.yields.map(v => `${t(v.name)} ×${v.quantity}`).join(", ")}` : ""} confirmLabel={t("Dismantle")} cancelLabel={t("Cancel")} tone="danger" busy={busy} onConfirm={() => void dismantle()} onCancel={() => { if (!busy) setConfirm(false); }} />
 </section>;
}
