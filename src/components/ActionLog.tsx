"use client";

import * as React from "react";
import type { ActionLogEntry } from "@/types";
import styles from "@/styles/ActionLog.module.css";

type ActionLogProps = {
    entries: ActionLogEntry[];
    compact?: boolean;
    expanded?: boolean;
    onExpandedChange?: (expanded: boolean) => void;
};

function toneClass(tone: ActionLogEntry["tone"]) {
    switch (tone) {
        case "success":
            return styles.entrySuccess;
        case "warning":
            return styles.entryWarning;
        case "danger":
            return styles.entryDanger;
        default:
            return styles.entryInfo;
    }
}

function categoryLabel(category: ActionLogEntry["category"]) {
    switch (category) {
        case "counterattack":
            return "Counter";
        case "artifact":
            return "Artifact";
        case "equipment":
            return "Equipment";
        case "resource":
            return "Loot";
        case "barrel":
            return "Barrel";
        case "portal":
            return "Portal";
        case "defeat":
            return "Defeat";
        case "movement":
            return "Move";
        case "attack":
            return "Combat";
        case "effect":
            return "Effect";
        case "turn":
            return "Turn";
        case "match":
            return "Match";
        default:
            return "Info";
    }
}

export default function ActionLog({
    entries,
    compact = false,
    expanded,
    onExpandedChange,
}: ActionLogProps) {
    const [internalCollapsed, setInternalCollapsed] = React.useState(compact);
    const listRef = React.useRef<HTMLOListElement | null>(null);
    const isControlled = typeof expanded === "boolean";
    const collapsed = isControlled ? !expanded : internalCollapsed;
    const latestEntry = entries[entries.length - 1];

    React.useEffect(() => {
        if (compact && !isControlled) setInternalCollapsed(true);
    }, [compact, isControlled]);

    React.useEffect(() => {
        if (collapsed) return;
        const list = listRef.current;
        if (!list) return;

        const frameId = window.requestAnimationFrame(() => {
            list.scrollTop = list.scrollHeight;
        });

        return () => window.cancelAnimationFrame(frameId);
    }, [collapsed, entries.length]);

    const handleToggle = React.useCallback(() => {
        const nextExpanded = collapsed;
        if (isControlled) {
            onExpandedChange?.(nextExpanded);
            return;
        }
        setInternalCollapsed(!collapsed);
        onExpandedChange?.(nextExpanded);
    }, [collapsed, isControlled, onExpandedChange]);

    if (entries.length === 0) return null;

    return (
        <section
            className={`${styles.panel} ${compact ? styles.panelCompact : ""} ${
                collapsed ? styles.panelCollapsed : ""
            }`}
            aria-label="Action log"
        >
            <header
                className={`${styles.header} ${
                    collapsed ? styles.headerCollapsed : ""
                }`}
            >
                <div className={styles.heading}>
                    <p className={styles.eyebrow}>Action Log</p>
                    {collapsed ? (
                        <p className={styles.collapsedMessage}>
                            {latestEntry?.message ?? "No events yet"}
                        </p>
                    ) : (
                        <p className={styles.subtitle}>Latest match events</p>
                    )}
                </div>
                <button
                    type="button"
                    className={styles.toggle}
                    onClick={handleToggle}
                    aria-expanded={!collapsed}
                >
                    {collapsed ? (compact ? "Open" : `Show ${entries.length}`) : "Hide"}
                </button>
            </header>

            {!collapsed && (
                <ol ref={listRef} className={styles.list}>
                    {entries.map((entry) => (
                        <li
                            key={entry.id}
                            className={`${styles.entry} ${toneClass(entry.tone)}`}
                        >
                            <span className={styles.category}>
                                {categoryLabel(entry.category)}
                            </span>
                            <span className={styles.message}>
                                {entry.message}
                            </span>
                        </li>
                    ))}
                </ol>
            )}
        </section>
    );
}
