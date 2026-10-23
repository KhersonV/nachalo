"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useRouter } from "next/navigation";
import { API_BASE } from "@/utils/serviceUrls";
import type { GameLanguage } from "@/i18n/hexGame";
import styles from "@/styles/GameChat.module.css";

export type OnlinePlayer = { userId: number; name: string };
type Relation = "none" | "friend" | "outgoing" | "incoming" | "self" | "loading";
type Props = {
    userId: number;
    token: string;
    language: GameLanguage;
    mutedIds: number[];
    onPrivate: (player: OnlinePlayer) => void;
    onAddToConversation?: (player: OnlinePlayer) => void;
    onMutesChanged: (ids: number[]) => void;
};

const labels = {
    en: { online: "Online", search: "Find player", empty: "Nobody is online", noMatch: "No players found", loading: "Loading…", error: "Online players unavailable", private: "Private message", mute: "Mute", unmute: "Unmute", profile: "Open profile", add: "Add friend", remove: "Remove friend", outgoing: "Cancel friend request", incoming: "Accept friend request", self: "You", actionError: "Action failed" },
    ru: { online: "В сети", search: "Найти игрока", empty: "Игроков в сети нет", noMatch: "Никого не найдено", loading: "Загрузка…", error: "Список игроков недоступен", private: "Написать приватно", mute: "В мут", unmute: "Убрать из мута", profile: "Перейти в профиль", add: "Добавить в друзья", remove: "Удалить из друзей", outgoing: "Отменить заявку", incoming: "Принять заявку", self: "Вы", actionError: "Не удалось выполнить действие" },
    de: { online: "Online", search: "Spieler suchen", empty: "Niemand ist online", noMatch: "Keine Spieler gefunden", loading: "Lädt…", error: "Spielerliste nicht verfügbar", private: "Privat schreiben", mute: "Stummschalten", unmute: "Stummschaltung aufheben", profile: "Profil öffnen", add: "Freund hinzufügen", remove: "Freund entfernen", outgoing: "Anfrage zurückziehen", incoming: "Anfrage annehmen", self: "Du", actionError: "Aktion fehlgeschlagen" },
};

export default function OnlinePlayersPanel({ userId, token, language, mutedIds, onPrivate, onAddToConversation, onMutesChanged }: Props) {
    const text = labels[language];
    const router = useRouter();
    const [players, setPlayers] = useState<OnlinePlayer[]>([]);
    const [search, setSearch] = useState("");
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");
    const [selected, setSelected] = useState<OnlinePlayer | null>(null);
    const [menuPosition, setMenuPosition] = useState({ left: 0, top: 0 });
    const [relation, setRelation] = useState<Relation>("loading");
    const [busy, setBusy] = useState(false);
    const [actionError, setActionError] = useState("");
    const menuRef = useRef<HTMLDivElement>(null);
    const requestSerial = useRef(0);
    const presenceVersion = useRef(0);

    const closeMenu = useCallback(() => {
        requestSerial.current++;
        setSelected(null);
        setActionError("");
    }, []);

    const refresh = useCallback(async () => {
        const version = presenceVersion.current;
        try {
            const response = await fetch(`${API_BASE}/game/chat/online`, { headers: { Authorization: `Bearer ${token}` }, cache: "no-store" });
            if (!response.ok) throw new Error("online unavailable");
            const data = await response.json() as { players?: OnlinePlayer[] };
            const unique = new Map<number, OnlinePlayer>();
            for (const player of data.players ?? []) {
                if (Number.isSafeInteger(player.userId) && player.userId > 0 && typeof player.name === "string") unique.set(player.userId, player);
            }
            if (version === presenceVersion.current) setPlayers([...unique.values()].sort((a, b) => a.name.localeCompare(b.name, language) || a.userId - b.userId));
            setError("");
        } catch { setError(text.error); }
        finally { setLoading(false); }
    }, [language, text.error, token]);

    useEffect(() => {
        void refresh();
        const onOpen = () => void refresh();
        const onFocus = () => void refresh();
        const onPresence = (event: Event) => {
            const frame = (event as CustomEvent<{ type?: string; payload?: OnlinePlayer }>).detail;
            const player = frame?.payload;
            if (!Number.isSafeInteger(player?.userId) || !player?.userId) return;
            presenceVersion.current++;
            setError("");
            setLoading(false);
            if (frame.type === "presence.leave") {
                setPlayers(previous => previous.filter(item => item.userId !== player.userId));
                setSelected(current => current?.userId === player.userId ? null : current);
            } else if (frame.type === "presence.join" && typeof player.name === "string") {
                setPlayers(previous => [...previous.filter(item => item.userId !== player.userId), player].sort((a, b) => a.name.localeCompare(b.name, language) || a.userId - b.userId));
            }
        };
        window.addEventListener("nachalo:chat-socket-open", onOpen);
        window.addEventListener("nachalo:presence-event", onPresence);
        window.addEventListener("focus", onFocus);
        const reconcile = window.setInterval(() => void refresh(), 60000);
        return () => {
            window.clearInterval(reconcile);
            window.removeEventListener("nachalo:chat-socket-open", onOpen);
            window.removeEventListener("nachalo:presence-event", onPresence);
            window.removeEventListener("focus", onFocus);
        };
    }, [language, refresh]);

    useEffect(() => {
        if (!selected) return;
        const onPointer = (event: PointerEvent) => { if (!menuRef.current?.contains(event.target as Node) && !(event.target as Element).closest("[data-online-player]")) closeMenu(); };
        const onKey = (event: KeyboardEvent) => { if (event.key === "Escape") closeMenu(); };
        const onScroll = (event: Event) => { if (!menuRef.current?.contains(event.target as Node)) closeMenu(); };
        document.addEventListener("pointerdown", onPointer);
        document.addEventListener("keydown", onKey);
        window.addEventListener("scroll", onScroll, true);
        return () => { document.removeEventListener("pointerdown", onPointer); document.removeEventListener("keydown", onKey); window.removeEventListener("scroll", onScroll, true); };
    }, [selected, closeMenu]);

    const filtered = useMemo(() => players.filter(player => player.name.toLocaleLowerCase(language).includes(search.trim().toLocaleLowerCase(language))), [players, search, language]);

    const openMenu = (player: OnlinePlayer, button: HTMLButtonElement) => {
        requestSerial.current++;
        const serial = requestSerial.current;
        const rect = button.getBoundingClientRect();
        const width = Math.min(206, window.innerWidth - 16);
        const height = Math.min(260, window.innerHeight - 16);
        setMenuPosition({ left: Math.max(8, Math.min(rect.left, window.innerWidth - width - 8)), top: Math.max(8, Math.min(rect.bottom + 4, window.innerHeight - height - 8)) });
        setSelected(player);
        setActionError("");
        setRelation(player.userId === userId ? "self" : "loading");
        if (player.userId === userId) return;
        void fetch(`${API_BASE}/game/profile/${player.userId}`, { headers: { Authorization: `Bearer ${token}` }, cache: "no-store" })
            .then(response => { if (!response.ok) throw new Error("profile unavailable"); return response.json() as Promise<{ data?: { friendRelation?: Relation } }>; })
            .then(data => { if (requestSerial.current === serial) setRelation(data.data?.friendRelation ?? "none"); })
            .catch(() => { if (requestSerial.current === serial) setActionError(text.actionError); });
    };

    const changeMute = async () => {
        if (!selected || selected.userId === userId || busy) return;
        const serial = requestSerial.current;
        const nextMuted = !mutedIds.includes(selected.userId);
        setBusy(true);
        setActionError("");
        try {
            const response = await fetch(`${API_BASE}/game/chat/mutes/${selected.userId}`, { method: nextMuted ? "PUT" : "DELETE", headers: { Authorization: `Bearer ${token}` } });
            if (!response.ok) throw new Error("mute failed");
            const next = nextMuted ? [...mutedIds, selected.userId] : mutedIds.filter(id => id !== selected.userId);
            onMutesChanged(next);
        } catch { if (requestSerial.current === serial) setActionError(text.actionError); }
        finally { setBusy(false); }
    };

    const changeFriend = async () => {
        if (!selected || selected.userId === userId || relation === "loading" || busy) return;
        const serial = requestSerial.current;
        setBusy(true);
        setActionError("");
        try {
            const id = selected.userId;
            const path = relation === "friend" ? `/game/friends/${id}` : relation === "outgoing" ? `/game/friends/requests/${id}` : relation === "incoming" ? `/game/friends/requests/${id}/accept` : "/game/friends/add";
            const method = relation === "friend" || relation === "outgoing" ? "DELETE" : "POST";
            const response = await fetch(`${API_BASE}${path}`, { method, headers: { Authorization: `Bearer ${token}`, ...(relation === "none" ? { "Content-Type": "application/json" } : {}) }, ...(relation === "none" ? { body: JSON.stringify({ friendUserId: id }) } : {}) });
            if (!response.ok) throw new Error("friend action failed");
            if (requestSerial.current === serial) setRelation(relation === "friend" || relation === "outgoing" ? "none" : relation === "incoming" ? "friend" : "outgoing");
        } catch { if (requestSerial.current === serial) setActionError(text.actionError); }
        finally { setBusy(false); }
    };

    const openProfile = () => {
        if (!selected) return;
        const id = selected.userId;
        closeMenu();
        const game = window.location.pathname === "/game";
        const instance = new URLSearchParams(window.location.search).get("instance_id");
        router.push(`/profile?view=${id}&from=${game ? "game" : "lobby"}${game && instance ? `&instance_id=${encodeURIComponent(instance)}` : ""}`);
    };

    return <aside className={styles.onlinePanel} aria-label={text.online}>
        <div className={styles.onlineHead}><strong>{text.online}</strong><span>{players.length}</span></div>
        <input className={styles.onlineSearch} type="search" value={search} onChange={event => setSearch(event.target.value)} placeholder={text.search} aria-label={text.search} />
        <div className={styles.onlineList}>
            {loading ? <p className={styles.onlineState}>{text.loading}</p> : error ? <p className={styles.onlineState} role="alert">{error}</p> : filtered.length === 0 ? <p className={styles.onlineState}>{players.length ? text.noMatch : text.empty}</p> : filtered.map(player =>
                <button key={player.userId} data-online-player type="button" className={styles.onlinePlayer} aria-haspopup="menu" aria-expanded={selected?.userId === player.userId} onClick={event => openMenu(player, event.currentTarget)}>
                    <span className={styles.onlineDot} /><span className={styles.onlineName}>{player.name}{player.userId === userId && <small> · {text.self}</small>}</span>
                </button>)}
        </div>
        {selected && typeof document !== "undefined" && createPortal(<div className={styles.playerMenu} ref={menuRef} role="menu" aria-label={selected.name} style={{ left: menuPosition.left, top: menuPosition.top }}>
            <strong className={styles.menuTitle}>{selected.name}</strong>
            {selected.userId !== userId && <button type="button" role="menuitem" onClick={() => { onPrivate(selected); closeMenu(); }}>{text.private}</button>}
            {selected.userId !== userId && onAddToConversation && <button type="button" role="menuitem" onClick={() => { onAddToConversation(selected); closeMenu(); }}>Добавить в новую беседу</button>}
            {selected.userId !== userId && <button type="button" role="menuitem" disabled={busy} onClick={() => void changeMute()}>{mutedIds.includes(selected.userId) ? text.unmute : text.mute}</button>}
            <button type="button" role="menuitem" onClick={openProfile}>{text.profile}</button>
            {selected.userId !== userId && <button type="button" role="menuitem" disabled={busy || relation === "loading"} onClick={() => void changeFriend()}>{relation === "friend" ? text.remove : relation === "outgoing" ? text.outgoing : relation === "incoming" ? text.incoming : text.add}</button>}
            {actionError && <p role="alert" className={styles.menuError}>{actionError}</p>}
        </div>, document.body)}
    </aside>;
}
