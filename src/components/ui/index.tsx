"use client";

import { useEffect, useId, useRef, type ButtonHTMLAttributes, type HTMLAttributes, type InputHTMLAttributes, type ReactNode } from "react";
import styles from "./ui.module.css";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";

type PageHeaderProps = HTMLAttributes<HTMLElement> & { title: string; description?: string; actions?: ReactNode };
export function PageHeader({ title, description, actions, className = "", ...props }: PageHeaderProps) {
    const { language } = useGameLanguage();
    return <header className={`${styles.pageHeader} ${className}`} {...props}><div><h1>{uiText(language, title)}</h1>{description && <p>{uiText(language, description)}</p>}</div>{actions && <div className={styles.actions}>{actions}</div>}</header>;
}

type PanelProps = HTMLAttributes<HTMLElement> & { title?: string; as?: "section" | "article" | "div" };
export function Panel({ title, as: Tag = "section", className = "", children, ...props }: PanelProps) {
    const { language } = useGameLanguage();
    return <Tag className={`${styles.panel} ${className}`} {...props}>{title && <h2>{uiText(language, title)}</h2>}{children}</Tag>;
}
export const Card = Panel;

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "secondary" | "danger"; busy?: boolean };
export function Button({ variant = "secondary", busy = false, disabled, className = "", children, type = "button", ...props }: ButtonProps) {
    return <button type={type} disabled={disabled || busy} aria-busy={busy || undefined} className={`${styles.button} ${styles[variant]} ${className}`} {...props}>{children}</button>;
}

type Tab = { id: string; label: string; disabled?: boolean };
type TabsProps = { tabs: readonly Tab[]; activeId: string; onChange: (id: string) => void; label: string; id?: string };
export function Tabs({ tabs, activeId, onChange, label, id = "tabs" }: TabsProps) {
    const { language } = useGameLanguage();
    const refs = useRef<(HTMLButtonElement | null)[]>([]);
    function move(index: number, direction: number) {
        for (let step = 1; step <= tabs.length; step++) {
            const next = (index + direction * step + tabs.length * step) % tabs.length;
            if (!tabs[next].disabled) { onChange(tabs[next].id); refs.current[next]?.focus(); break; }
        }
    }
    return <div role="tablist" aria-label={uiText(language, label)} className={styles.tabs}>{tabs.map((tab, index) => <button key={tab.id} ref={el => { refs.current[index] = el; }} id={`${id}-${tab.id}`} role="tab" type="button" aria-selected={activeId === tab.id} tabIndex={activeId === tab.id ? 0 : -1} disabled={tab.disabled} className={styles.tab} onClick={() => onChange(tab.id)} onKeyDown={event => { if (event.key === "ArrowRight") { event.preventDefault(); move(index, 1); } if (event.key === "ArrowLeft") { event.preventDefault(); move(index, -1); } }}>{uiText(language, tab.label)}</button>)}</div>;
}

type InputProps = InputHTMLAttributes<HTMLInputElement> & { label: string; error?: string; hint?: string };
export function Input({ label, error, hint, id, className = "", ...props }: InputProps) {
    const { language } = useGameLanguage();
    const generatedId = useId();
    const inputId = id || generatedId;
    return <div className={`${styles.field} ${className}`}><label htmlFor={inputId}>{uiText(language, label)}</label><input id={inputId} aria-invalid={!!error} aria-describedby={error ? `${inputId}-error` : hint ? `${inputId}-hint` : undefined} {...props} />{error ? <span id={`${inputId}-error`} className={styles.fieldError}>{uiText(language, error)}</span> : hint ? <span id={`${inputId}-hint`} className={styles.hint}>{uiText(language, hint)}</span> : null}</div>;
}

type StatusProps = HTMLAttributes<HTMLDivElement> & { tone?: "info" | "success" | "warning" | "error"; title?: string };
export function StatusMessage({ tone = "info", title, children, className = "", ...props }: StatusProps) {
    return <div role={tone === "error" ? "alert" : "status"} className={`${styles.status} ${styles[tone]} ${className}`} {...props}>{title && <strong>{title}</strong>}{children && <div>{children}</div>}</div>;
}

type StateProps = { title: string; description?: string; action?: ReactNode };
export function LoadingState({ title = "Loading…", description }: Partial<StateProps>) {
    const { language } = useGameLanguage();
    return <div role="status" aria-live="polite" className={styles.state}><span className={styles.loader} aria-hidden="true" /><strong>{uiText(language, title)}</strong>{description && <p>{uiText(language, description)}</p>}</div>;
}
export function EmptyState({ title, description, action }: StateProps) {
    const { language } = useGameLanguage();
    return <div className={styles.state}><strong>{uiText(language, title)}</strong>{description && <p>{uiText(language, description)}</p>}{action}</div>;
}
export function ErrorState({ title, description, action }: StateProps) {
    const { language } = useGameLanguage();
    return <div role="alert" className={`${styles.state} ${styles.errorState}`}><strong>{uiText(language, title)}</strong>{description && <p>{uiText(language, description)}</p>}{action}</div>;
}

type ConfirmDialogProps = { open: boolean; title: string; description: string; confirmLabel: string; cancelLabel?: string; tone?: "primary" | "danger"; busy?: boolean; onConfirm: () => void; onCancel: () => void };
export function ConfirmDialog({ open, title, description, confirmLabel, cancelLabel = "Cancel", tone = "primary", busy, onConfirm, onCancel }: ConfirmDialogProps) {
    const { language } = useGameLanguage();
    const id = useId();
    const dialogRef = useRef<HTMLDialogElement>(null);
    useEffect(() => {
        const dialog = dialogRef.current;
        if (!dialog) return;
        if (open && !dialog.open) { dialog.showModal(); dialog.querySelector<HTMLButtonElement>('[data-confirm]')?.focus(); }
        if (!open && dialog.open) dialog.close();
    }, [open]);
    return <dialog ref={dialogRef} className={styles.dialog} aria-labelledby={`${id}-title`} aria-describedby={`${id}-description`} onCancel={event => { event.preventDefault(); onCancel(); }}><h2 id={`${id}-title`}>{uiText(language, title)}</h2><p id={`${id}-description`}>{uiText(language, description)}</p><div className={styles.dialogActions}><Button onClick={onCancel} disabled={busy}>{uiText(language, cancelLabel)}</Button><Button data-confirm variant={tone} busy={busy} onClick={onConfirm}>{uiText(language, confirmLabel)}</Button></div></dialog>;
}
