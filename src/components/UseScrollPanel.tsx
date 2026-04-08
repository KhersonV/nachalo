"use client";

import React from "react";
import { useAuth } from "../contexts/AuthContext";
import styles from "../styles/GameController.module.css";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8001";

interface Props {
    instanceId: string;
}

export default function UseScrollPanel({ instanceId }: Props) {
    const { user } = useAuth();
    const [open, setOpen] = React.useState(false);
    const [loading, setLoading] = React.useState(false);
    const [items, setItems] = React.useState<any[]>([]);
    const [counts, setCounts] = React.useState<Record<string, number>>({});
    const [message, setMessage] = React.useState<string | null>(null);

    const loadData = React.useCallback(async () => {
        if (!user) return;
        setLoading(true);
        try {
            const [catalogRes, playerRes] = await Promise.all([
                fetch(`${API_BASE}/game/shop/items`),
                fetch(`${API_BASE}/game/player/${user.id}`, {
                    headers: user.token
                        ? { Authorization: `Bearer ${user.token}` }
                        : {},
                }),
            ]);
            const catalog = catalogRes.ok
                ? await catalogRes.json()
                : { items: [] };
            const player = playerRes.ok ? await playerRes.json() : null;
            const inv = player?.inventory
                ? typeof player.inventory === "string"
                    ? JSON.parse(player.inventory)
                    : player.inventory
                : {};
            const scrolls = (catalog.items || []).filter(
                (it: any) => it.category === "scroll",
            );
            setItems(scrolls);
            const nextCounts: Record<string, number> = {};
            for (const s of scrolls) {
                const key = s.inventoryKey || `scroll_${s.id}`;
                const entry = inv?.[key];
                let c = 0;
                if (entry) {
                    const raw = entry.item_count ?? entry.count ?? 0;
                    c =
                        typeof raw === "number"
                            ? raw
                            : parseInt(String(raw)) || 0;
                }
                nextCounts[s.type] = c;
            }
            setCounts(nextCounts);
        } catch (e) {
            console.error(e);
        } finally {
            setLoading(false);
        }
    }, [user]);

    React.useEffect(() => {
        if (open) loadData();
    }, [open, loadData]);

    const handleUse = React.useCallback(
        async (type: string) => {
            if (!user?.token) return;
            setLoading(true);
            setMessage(null);
            try {
                const res = await fetch(
                    `${API_BASE}/game/match/${instanceId}/use-scroll`,
                    {
                        method: "POST",
                        headers: {
                            "Content-Type": "application/json",
                            Authorization: `Bearer ${user.token}`,
                        },
                        body: JSON.stringify({ scroll_type: type }),
                    },
                );
                if (!res.ok) {
                    const t = await res.text();
                    throw new Error(t || "Use failed");
                }
                const data = await res.json();
                setMessage(
                    `Revealed ${data.axis.toUpperCase()}: ${data.value}`,
                );
                // decrement local count
                setCounts((prev) => ({
                    ...(prev || {}),
                    [type]: Math.max(0, (prev[type] || 1) - 1),
                }));
            } catch (e: any) {
                setMessage(e?.message || "Failed to use scroll");
            } finally {
                setLoading(false);
            }
        },
        [instanceId, user],
    );

    return (
        <div>
            <button
                type="button"
                className={styles.useScrollFab}
                onClick={() => setOpen((v) => !v)}
                title={open ? "Close scrolls" : "Use scroll"}
            >
                📜
            </button>

            {open && (
                <div
                    className={styles.useScrollPanel}
                    role="dialog"
                    aria-label="Use scrolls"
                >
                    <div className={styles.useScrollHeader}>
                        <strong>Scrolls</strong>
                        <button onClick={() => setOpen(false)}>×</button>
                    </div>
                    {loading && <div>Loading...</div>}
                    {message && (
                        <div className={styles.useScrollMessage}>{message}</div>
                    )}
                    <div className={styles.useScrollList}>
                        {items.map((it) => (
                            <div key={it.type} className={styles.useScrollItem}>
                                <img
                                    src={it.image}
                                    alt={it.name}
                                    className={styles.useScrollImage}
                                />
                                <div className={styles.useScrollMeta}>
                                    <div>{it.name}</div>
                                    <div className={styles.useScrollCount}>
                                        Qty: {counts[it.type] || 0}
                                    </div>
                                </div>
                                <div>
                                    <button
                                        disabled={
                                            loading ||
                                            (counts[it.type] || 0) <= 0
                                        }
                                        onClick={() => handleUse(it.type)}
                                    >
                                        Use
                                    </button>
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
            )}
        </div>
    );
}
