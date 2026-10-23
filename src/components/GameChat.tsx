"use client";

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { createPortal } from "react-dom";
import { useAuth } from "@/contexts/AuthContext";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { API_BASE, MATCHMAKING_BASE, WS_URL } from "@/utils/serviceUrls";
import OnlinePlayersPanel, { type OnlinePlayer } from "@/components/OnlinePlayersPanel";
import ChatEmoteMessage, { EmoteImage } from "@/components/ChatEmoteMessage";
import { chatEmotes, type ChatEmoteCategory } from "@/utils/chatEmotes";
import styles from "@/styles/GameChat.module.css";

type Channel = "MAIN" | "GROUP" | "TRADE";
type ChatMessage = {
    id: number;
    channel: Channel | "PRIVATE" | "SYSTEM";
    senderUserId: number;
    senderNickname: string;
    body: string;
    createdAt: string;
    partyId?: string;
    recipientUserId?: number;
    conversationId?: number;
};
type ChatConversation = { id: number; kind: "DIRECT" | "MULTI"; participants: OnlinePlayer[]; unreadCount: number; createdAt: string };
type PrivateNotice = { messageId: number; conversationId: number; senderId: number; sender: string; createdAt: string };
type HistoryPage = { messages: ChatMessage[]; nextBeforeId: number | null };
type Lists = Record<Channel, ChatMessage[]>;
type Flags = Record<Channel, boolean>;
type Cursors = Record<Channel, number | null>;
const channels: Channel[] = ["MAIN", "GROUP", "TRADE"];
const emptyLists = (): Lists => ({ MAIN: [], GROUP: [], TRADE: [] });
const emptyFlags = (): Flags => ({ MAIN: false, GROUP: false, TRADE: false });
const emptyCursors = (): Cursors => ({ MAIN: null, GROUP: null, TRADE: null });
const emojiCategories = [
    { key: "faces", icon: "😀", emojis: ["😀", "😁", "😂", "😊", "😎", "😍", "😢", "😭", "😡", "🤔", "😉", "😴"] },
    { key: "gestures", icon: "👍", emojis: ["👍", "👎", "👏", "🙌", "👋", "🤝", "💪", "👌", "🙏"] },
    { key: "hearts", icon: "❤️", emojis: ["❤️", "💙", "💚", "💜", "💔", "✨", "⭐", "🔥", "💯"] },
    { key: "fantasy", icon: "⚔️", emojis: ["⚔️", "🛡️", "🏹", "🗡️", "🪄", "🧙", "🐉", "💀", "👑", "💎", "🏆"] },
] as const;
type EmojiCategory = typeof emojiCategories[number]["key"];
type PickerSection = ChatEmoteCategory | "emoji";

const emoteLabels = {
    en: { classic: "Classic", fantasy: "Fantasy", emoji: "Emoji", animation: "Animate emotes", unavailable: "Emote artwork coming soon" },
    ru: { classic: "Обычные", fantasy: "Фэнтези", emoji: "Emoji", animation: "Анимация смайлов", unavailable: "Изображения смайлов появятся позже" },
    de: { classic: "Klassisch", fantasy: "Fantasy", emoji: "Emoji", animation: "Emotes animieren", unavailable: "Emote-Grafiken folgen später" },
};
const emotePreferenceKey = "nachalo.chat.animateEmotes";

const labels = {
    en: { title: "Chat", main: "Main", group: "Group", trade: "Trade", private: "Private", send: "Send", placeholder: "Write a message…", older: "Load older", empty: "No messages yet", noParty: "Join a Party to use Group chat", unavailable: "Party status unavailable", latest: "Latest messages", sending: "Sending…", invalid: "Enter 1–1000 characters", limit: "Please wait before sending again", denied: "You are no longer in this Party", failure: "Chat is temporarily unavailable", emoji: "Choose emoji", faces: "Faces", gestures: "Gestures", hearts: "Hearts and symbols", fantasy: "Fantasy and combat", privateTo: "Private chat with", backToChannels: "Back to channels" },
    ru: { title: "Чат", main: "Основной", group: "Группа", trade: "Торговый", private: "Лично", send: "Отправить", placeholder: "Напишите сообщение…", older: "Ранее", empty: "Сообщений пока нет", noParty: "Вступите в группу, чтобы писать сюда", unavailable: "Не удалось проверить группу", latest: "К новым сообщениям", sending: "Отправка…", invalid: "Введите от 1 до 1000 символов", limit: "Подождите перед следующей отправкой", denied: "Вы больше не состоите в этой группе", failure: "Чат временно недоступен", emoji: "Выбрать эмодзи", faces: "Лица", gestures: "Жесты", hearts: "Сердца и символы", fantasy: "Фэнтези и бой", privateTo: "Личный чат с", backToChannels: "К каналам" },
    de: { title: "Chat", main: "Allgemein", group: "Gruppe", trade: "Handel", private: "Privat", send: "Senden", placeholder: "Nachricht schreiben…", older: "Ältere laden", empty: "Noch keine Nachrichten", noParty: "Tritt einer Gruppe bei, um hier zu schreiben", unavailable: "Gruppenstatus nicht verfügbar", latest: "Neueste Nachrichten", sending: "Wird gesendet…", invalid: "1–1000 Zeichen eingeben", limit: "Bitte vor der nächsten Nachricht warten", denied: "Du bist nicht mehr in dieser Gruppe", failure: "Chat ist vorübergehend nicht verfügbar", emoji: "Emoji auswählen", faces: "Gesichter", gestures: "Gesten", hearts: "Herzen und Symbole", fantasy: "Fantasy und Kampf", privateTo: "Privatchat mit", backToChannels: "Zurück zu Kanälen" },
};

function mergeMessages(previous: ChatMessage[], incoming: ChatMessage[]): ChatMessage[] {
    const byId = new Map(previous.map(message => [message.id, message]));
    for (const message of incoming) if (Number.isSafeInteger(message.id) && message.id > 0) byId.set(message.id, message);
    return [...byId.values()].sort((a, b) => a.id - b.id);
}

function isChatMessage(value: unknown): value is ChatMessage {
    if (!value || typeof value !== "object") return false;
    const message = value as Partial<ChatMessage>;
    return Number.isSafeInteger(message.id) && typeof message.channel === "string" && typeof message.body === "string" && typeof message.senderNickname === "string" && typeof message.createdAt === "string";
}

export default function GameChat() {
    const { user } = useAuth();
    const pathname = usePathname();
    const { language } = useGameLanguage();
    const text = labels[language];
    const emoteText = emoteLabels[language];
    const match = pathname === "/game";
    const visible = Boolean(user && (match || ["/mode", "/base", "/equipment", "/shop", "/market", "/profile", "/matches"].some(path => pathname === path || pathname.startsWith(path + "/"))));
    const [expanded, setExpanded] = useState(false);
    const [channel, setChannel] = useState<Channel>("MAIN");
    const [privateView, setPrivateView] = useState<ChatConversation | null>(null);
    const [conversations, setConversations] = useState<ChatConversation[]>([]);
    const [notices, setNotices] = useState<PrivateNotice[]>([]);
    const [creating, setCreating] = useState(false);
    const [selectedPlayers, setSelectedPlayers] = useState<OnlinePlayer[]>([]);
    const [playerSearch, setPlayerSearch] = useState("");
    const [playerResults, setPlayerResults] = useState<OnlinePlayer[]>([]);
    const [privateVersion, setPrivateVersion] = useState(0);
    const [mutedIds, setMutedIds] = useState<number[]>([]);
    const [draft, setDraft] = useState("");
    const [emojiOpen, setEmojiOpen] = useState(false);
    const [emojiCategory, setEmojiCategory] = useState<EmojiCategory>("faces");
    const [pickerSection, setPickerSection] = useState<PickerSection>("emoji");
    const [animateEmotes, setAnimateEmotes] = useState<boolean | null>(null);
    const [emojiPosition, setEmojiPosition] = useState({ left: 8, top: 8 });
    const [lists, setLists] = useState<Lists>(emptyLists);
    const [loaded, setLoaded] = useState<Flags>(emptyFlags);
    const [cursor, setCursor] = useState<Cursors>(emptyCursors);
    const [unread, setUnread] = useState<Record<Channel, number>>({ MAIN: 0, GROUP: 0, TRADE: 0 });
    const [partyId, setPartyId] = useState<string | null>(null);
    const [partyChecked, setPartyChecked] = useState(false);
    const [partyUnavailable, setPartyUnavailable] = useState(false);
    const [loading, setLoading] = useState<Channel | null>(null);
    const [sending, setSending] = useState(false);
    const [error, setError] = useState("");
    const [showLatest, setShowLatest] = useState(false);
    const listsRef = useRef(lists);
    const loadedRef = useRef(loaded);
    const cursorRef = useRef(cursor);
    const partyRef = useRef(partyId);
    const activeRef = useRef<{ channel: Channel; expanded: boolean; privateId?: number }>({ channel, expanded });
    const pendingHistory = useRef(new Set<Channel>());
    const sendingRef = useRef(false);
    const privateMessages = useRef(new Map<number, ChatMessage[]>());
    const privateCursors = useRef(new Map<number, number | null>());
    const privateLoaded = useRef(new Set<number>());
    const privatePending = useRef(new Set<number>());
    const conversationRefresh = useRef(0);
    const previousMutes = useRef<string | null>(null);
    const mutedRef = useRef(new Set<number>());
    const muteVersion = useRef(0);
    const scrollRef = useRef<HTMLDivElement>(null);
    const inputRef = useRef<HTMLTextAreaElement>(null);
    const emojiButtonRef = useRef<HTMLButtonElement>(null);
    const emojiPanelRef = useRef<HTMLDivElement>(null);
    const caretRef = useRef({ start: 0, end: 0 });
    const pendingCaretRef = useRef<number | null>(null);
    const stickToBottom = useRef(true);
    const restoreScroll = useRef<{ height: number; top: number } | null>(null);
    listsRef.current = lists;
    loadedRef.current = loaded;
    cursorRef.current = cursor;
    partyRef.current = partyId;
    activeRef.current = { channel, expanded, privateId: privateView?.id };

    useLayoutEffect(() => {
        const caret = pendingCaretRef.current;
        if (caret === null || !inputRef.current) return;
        pendingCaretRef.current = null;
        inputRef.current.focus({ preventScroll: true });
        inputRef.current.setSelectionRange(caret, caret);
        caretRef.current = { start: caret, end: caret };
    }, [draft]);

    useEffect(() => {
        if (!emojiOpen || !expanded || !visible) return;
        const onPointer = (event: PointerEvent) => {
            const target = event.target as Node;
            if (!emojiPanelRef.current?.contains(target) && !emojiButtonRef.current?.contains(target)) setEmojiOpen(false);
        };
        const onKey = (event: KeyboardEvent) => {
            if (event.key !== "Escape") return;
            event.preventDefault();
            event.stopPropagation();
            setEmojiOpen(false);
            inputRef.current?.focus({ preventScroll: true });
        };
        const onResize = () => setEmojiOpen(false);
        document.addEventListener("pointerdown", onPointer);
        document.addEventListener("keydown", onKey, true);
        window.addEventListener("resize", onResize);
        return () => {
            document.removeEventListener("pointerdown", onPointer);
            document.removeEventListener("keydown", onKey, true);
            window.removeEventListener("resize", onResize);
        };
    }, [emojiOpen, expanded, visible]);

    const changeMutes = useCallback((ids: number[]) => {
        muteVersion.current++;
        mutedRef.current = new Set(ids);
        setMutedIds(ids);
        setNotices(current => current.filter(item => !ids.includes(item.senderId)));
    }, []);

    const refreshConversations = useCallback(async () => {
        if (!user?.token) return;
        const serial = ++conversationRefresh.current;
        try {
            const response = await fetch(`${API_BASE}/game/chat/conversations`, { headers: { Authorization: `Bearer ${user.token}` }, cache: "no-store" });
            if (!response.ok) return;
            const data = await response.json() as { conversations: ChatConversation[] };
            if (serial === conversationRefresh.current) setConversations(data.conversations ?? []);
            return data.conversations ?? [];
        } catch { /* Keep currently loaded conversations. */ }
    }, [user?.token]);

    const markRead = useCallback(async (id: number) => {
        if (!user?.token) return;
        try {
            const response = await fetch(`${API_BASE}/game/chat/conversations/${id}/read`, { method: "POST", headers: { Authorization: `Bearer ${user.token}` } });
            if (response.ok) setConversations(current => current.map(item => item.id === id ? { ...item, unreadCount: 0 } : item));
        } catch { /* Recovered on refresh. */ }
    }, [user?.token]);

    const openConversation = useCallback((conversation: ChatConversation) => {
        setEmojiOpen(false);
        setPrivateView(conversation);
        setNotices(previous => previous.filter(item => item.conversationId !== conversation.id));
        setExpanded(true);
        stickToBottom.current = true;
        void markRead(conversation.id);
        window.setTimeout(() => inputRef.current?.focus(), 0);
    }, [markRead]);

    const createConversation = useCallback(async (players: OnlinePlayer[]) => {
        if (!user?.token || !players.length) return;
        try {
            const response = await fetch(`${API_BASE}/game/chat/conversations`, { method: "POST", headers: { Authorization: `Bearer ${user.token}`, "Content-Type": "application/json" }, body: JSON.stringify({ recipientIds: players.map(player => player.userId) }) });
            if (!response.ok) { setError(text.failure); return; }
            const conversation = await response.json() as ChatConversation;
            setConversations(current => [conversation, ...current.filter(item => item.id !== conversation.id)]);
            setCreating(false); setSelectedPlayers([]); setPlayerSearch("");
            openConversation(conversation);
        } catch { setError(text.failure); }
    }, [openConversation, text.failure, user?.token]);

    const openPrivate = useCallback((player: OnlinePlayer) => { void createConversation([player]); }, [createConversation]);

    const addParticipant = useCallback((player: OnlinePlayer) => {
        if (player.userId === user?.id) return;
        setCreating(true);
        setSelectedPlayers(current => current.some(item => item.userId === player.userId) ? current : [...current, player]);
        setExpanded(true);
    }, [user?.id]);

    useEffect(() => { if (!visible) setEmojiOpen(false); }, [visible]);

    useEffect(() => {
        const media = window.matchMedia("(prefers-reduced-motion: reduce)");
        const readPreference = () => {
            let saved: string | null = null;
            try { saved = window.localStorage.getItem(emotePreferenceKey); } catch { /* Use the system preference. */ }
            setAnimateEmotes(saved === "on" ? true : saved === "off" ? false : !media.matches);
        };
        readPreference();
        media.addEventListener("change", readPreference);
        return () => media.removeEventListener("change", readPreference);
    }, []);

    const changeAnimation = (enabled: boolean) => {
        setAnimateEmotes(enabled);
        try { window.localStorage.setItem(emotePreferenceKey, enabled ? "on" : "off"); } catch { /* Keep the current session setting. */ }
    };

    const updateList = useCallback((key: Channel, incoming: ChatMessage[]) => {
        const next = { ...listsRef.current, [key]: mergeMessages(listsRef.current[key], incoming) };
        listsRef.current = next;
        setLists(next);
    }, []);

    const loadHistory = useCallback(async (key: Channel, mode: "initial" | "older" | "recover") => {
        if (!user?.token || pendingHistory.current.has(key) || (key === "GROUP" && !partyRef.current)) return;
        const before = mode === "older" ? cursorRef.current[key] : null;
        if (mode === "older" && !before) return;
        pendingHistory.current.add(key);
        setLoading(key);
        const partyAtStart = key === "GROUP" ? partyRef.current : null;
        try {
            const newestKnown = listsRef.current[key].at(-1)?.id ?? 0;
            let pageCursor = before;
            let page: HistoryPage;
            do {
                const query = new URLSearchParams({ channel: key, limit: "40" });
                if (pageCursor) query.set("beforeId", String(pageCursor));
                const response = await fetch(`${API_BASE}/game/chat/messages?${query}`, {
                    headers: { Authorization: `Bearer ${user.token}` }, cache: "no-store",
                });
                if (!response.ok) {
                    if (key === "GROUP" && response.status === 403) {
                        setPartyId(null);
                        partyRef.current = null;
                        setError(text.denied);
                    } else setError(response.status === 429 ? text.limit : text.failure);
                    return;
                }
                page = await response.json() as HistoryPage;
                if (key === "GROUP" && partyRef.current !== partyAtStart) return;
                const valid = Array.isArray(page.messages) ? page.messages.filter(isChatMessage).filter(message => message.channel === key) : [];
                updateList(key, valid);
                pageCursor = page.nextBeforeId;
                if (mode !== "recover") {
                    const next = { ...cursorRef.current, [key]: pageCursor };
                    cursorRef.current = next;
                    setCursor(next);
                }
                if (mode !== "recover" || !newestKnown || !pageCursor || !valid.length || valid[valid.length - 1].id <= newestKnown) break;
            } while (true);
            const nextLoaded = { ...loadedRef.current, [key]: true };
            loadedRef.current = nextLoaded;
            setLoaded(nextLoaded);
            setError("");
        } catch {
            setError(text.failure);
        } finally {
            pendingHistory.current.delete(key);
            setLoading(current => current === key ? null : current);
            if (key === "GROUP" && partyRef.current && partyRef.current !== partyAtStart) {
                window.setTimeout(() => void loadHistory("GROUP", "initial"), 0);
            }
        }
    }, [text.denied, text.failure, text.limit, updateList, user?.token]);

    const loadPrivateHistory = useCallback(async (conversationID: number, mode: "initial" | "older" | "recover") => {
        if (!user?.token || conversationID <= 0 || privatePending.current.has(conversationID)) return;
        const before = mode === "older" ? privateCursors.current.get(conversationID) : null;
        if (mode === "older" && !before) return;
        privatePending.current.add(conversationID);
        try {
            const newestKnown = privateMessages.current.get(conversationID)?.at(-1)?.id ?? 0;
            let pageCursor = before;
            do {
                const query = new URLSearchParams({ limit: "40" });
                if (pageCursor) query.set("beforeId", String(pageCursor));
                const response = await fetch(`${API_BASE}/game/chat/conversations/${conversationID}/messages?${query}`, { headers: { Authorization: `Bearer ${user.token}` }, cache: "no-store" });
                if (!response.ok) { setError(text.failure); return; }
                const page = await response.json() as HistoryPage;
                const valid = Array.isArray(page.messages) ? page.messages.filter(isChatMessage).filter(message => message.channel === "PRIVATE" && message.conversationId === conversationID) : [];
                privateMessages.current.set(conversationID, mergeMessages(privateMessages.current.get(conversationID) ?? [], valid));
                setPrivateVersion(value => value + 1);
                pageCursor = page.nextBeforeId;
                if (mode !== "recover") privateCursors.current.set(conversationID, pageCursor);
                if (mode !== "recover" || !newestKnown || !pageCursor || !valid.length || valid[valid.length - 1].id <= newestKnown) break;
            } while (true);
            privateLoaded.current.add(conversationID);
            setError("");
        } catch { setError(text.failure); }
        finally { privatePending.current.delete(conversationID); }
    }, [text.failure, user?.token]);

    const refreshLoaded = useCallback(() => {
        for (const key of channels) if (loadedRef.current[key]) void loadHistory(key, "recover");
        for (const id of privateLoaded.current) void loadPrivateHistory(id, "recover");
        void refreshConversations();
    }, [loadHistory, loadPrivateHistory, refreshConversations]);

    useEffect(() => {
        const key = [...mutedIds].sort((a, b) => a - b).join(",");
        if (previousMutes.current !== null && previousMutes.current !== key) {
            privateMessages.current.clear();
            privateCursors.current.clear();
            privateLoaded.current.clear();
            setPrivateVersion(value => value + 1);
            if (privateView) void loadPrivateHistory(privateView.id, "initial");
            void refreshConversations();
        }
        previousMutes.current = key;
    }, [loadPrivateHistory, mutedIds, privateView, refreshConversations]);

    const receive = useCallback((value: unknown) => {
        if (!isChatMessage(value)) return;
        const message = value;
        if (mutedRef.current.has(message.senderUserId)) return;
        if (message.channel === "PRIVATE") {
            const id = message.conversationId;
            if (id) {
                const previous = privateMessages.current.get(id) ?? [];
                if (previous.some(item => item.id === message.id)) return;
                privateMessages.current.set(id, mergeMessages(previous, [message]));
                setPrivateVersion(value => value + 1);
                void refreshConversations();
                if (message.senderUserId !== user?.id) {
                    if (activeRef.current.expanded && activeRef.current.privateId === id) void markRead(id);
                    else {
                        setConversations(current => current.map(item => item.id === id ? { ...item, unreadCount: item.unreadCount + 1 } : item));
                        setNotices(current => current.some(item => item.messageId === message.id) ? current : [...current, { messageId: message.id, conversationId: id, senderId: message.senderUserId, sender: message.senderNickname, createdAt: message.createdAt }]);
                    }
                }
            }
            return;
        }
        const key: Channel = message.channel === "SYSTEM" ? "MAIN" : message.channel;
        if (!channels.includes(key)) return;
        if (key === "GROUP" && message.partyId && partyRef.current !== message.partyId) {
            partyRef.current = message.partyId;
            setPartyId(message.partyId);
            const next = { ...listsRef.current, GROUP: [] };
            listsRef.current = next;
            setLists(next);
            const flags = { ...loadedRef.current, GROUP: false };
            loadedRef.current = flags;
            setLoaded(flags);
        }
        if (listsRef.current[key].some(item => item.id === message.id)) return;
        updateList(key, [message]);
        if (!activeRef.current.expanded || activeRef.current.channel !== key) {
            setUnread(previous => ({ ...previous, [key]: previous[key] + 1 }));
        }
    }, [markRead, refreshConversations, updateList, user?.id]);

    useEffect(() => {
        listsRef.current = emptyLists();
        loadedRef.current = emptyFlags();
        cursorRef.current = emptyCursors();
        partyRef.current = null;
        privateMessages.current.clear();
        privateCursors.current.clear();
        privateLoaded.current.clear();
        privatePending.current.clear();
        previousMutes.current = null;
        mutedRef.current.clear();
        setLists(listsRef.current);
        setLoaded(loadedRef.current);
        setCursor(cursorRef.current);
        setPartyId(null);
        setPartyChecked(false);
        setPartyUnavailable(false);
        setUnread({ MAIN: 0, GROUP: 0, TRADE: 0 });
        setPrivateView(null);
        setConversations([]);
        setNotices([]);
        setCreating(false);
        setSelectedPlayers([]);
        setPrivateVersion(0);
        setMutedIds([]);
        setDraft("");
        setEmojiOpen(false);
    }, [user?.id]);

    useEffect(() => {
        if (!visible || !user?.token) return;
        void refreshConversations();
        const refresh = () => void refreshConversations();
        window.addEventListener("focus", refresh);
        const interval = window.setInterval(refresh, 15000);
        return () => { window.removeEventListener("focus", refresh); window.clearInterval(interval); };
    }, [refreshConversations, user?.token, visible]);

    useEffect(() => {
        if (!creating || !user?.token) return;
        const controller = new AbortController();
        const timer = window.setTimeout(async () => {
            try {
                const response = await fetch(`${API_BASE}/game/chat/players?search=${encodeURIComponent(playerSearch)}`, { headers: { Authorization: `Bearer ${user.token}` }, signal: controller.signal });
                if (response.ok) { const data = await response.json() as { players: OnlinePlayer[] }; setPlayerResults(data.players ?? []); }
            } catch { /* Keep selected players. */ }
        }, 250);
        return () => { window.clearTimeout(timer); controller.abort(); };
    }, [creating, playerSearch, user?.token]);

    useEffect(() => {
        if (!visible || !user?.token) return;
        let stopped = false;
        const refreshMutes = async () => {
            const version = muteVersion.current;
            try {
                const response = await fetch(`${API_BASE}/game/chat/mutes`, { headers: { Authorization: `Bearer ${user.token}` }, cache: "no-store" });
                if (!response.ok) return;
                const data = await response.json() as { mutedUserIds?: number[] };
                if (!stopped && version === muteVersion.current) changeMutes((data.mutedUserIds ?? []).filter(id => Number.isSafeInteger(id) && id > 0));
            } catch { /* History and live delivery are still filtered on the server. */ }
        };
        void refreshMutes();
        window.addEventListener("focus", refreshMutes);
        window.addEventListener("nachalo:chat-socket-open", refreshMutes);
        return () => { stopped = true; window.removeEventListener("focus", refreshMutes); window.removeEventListener("nachalo:chat-socket-open", refreshMutes); };
    }, [changeMutes, user?.token, visible]);

    useEffect(() => {
        if (!visible || !user?.token || !user.id) return;
        let stopped = false;
        const refreshParty = async () => {
            try {
                const response = await fetch(`${MATCHMAKING_BASE}/matchmaking/party?player_id=${user.id}`, {
                    headers: { Authorization: `Bearer ${user.token}` }, cache: "no-store",
                });
                if (!response.ok) throw new Error("party unavailable");
                const state = await response.json() as { inParty?: boolean; partyId?: string; members?: { user_id: number }[] };
                if (stopped) return;
                const nextParty = state.inParty && state.partyId && state.members?.some(member => member.user_id === user.id) ? state.partyId : null;
                if (partyRef.current !== nextParty) {
                    partyRef.current = nextParty;
                    setPartyId(nextParty);
                    const next = { ...listsRef.current, GROUP: [] };
                    listsRef.current = next;
                    setLists(next);
                    const flags = { ...loadedRef.current, GROUP: false };
                    loadedRef.current = flags;
                    setLoaded(flags);
                    const cursors = { ...cursorRef.current, GROUP: null };
                    cursorRef.current = cursors;
                    setCursor(cursors);
                    setUnread(previous => ({ ...previous, GROUP: 0 }));
                }
                setPartyChecked(true);
                setPartyUnavailable(false);
            } catch {
                if (!stopped) { setPartyId(null); partyRef.current = null; setPartyChecked(true); setPartyUnavailable(true); }
            }
        };
        void refreshParty();
        const interval = window.setInterval(() => void refreshParty(), 10000);
        window.addEventListener("focus", refreshParty);
        return () => { stopped = true; window.clearInterval(interval); window.removeEventListener("focus", refreshParty); };
    }, [visible, user?.id, user?.token]);

    useEffect(() => {
        if (!visible || !user?.token) return;
        const onSharedMessage = (event: Event) => receive((event as CustomEvent<unknown>).detail);
        const onSharedOpen = () => refreshLoaded();
        window.addEventListener("nachalo:chat-message", onSharedMessage);
        window.addEventListener("nachalo:chat-socket-open", onSharedOpen);
        if (match) return () => {
            window.removeEventListener("nachalo:chat-message", onSharedMessage);
            window.removeEventListener("nachalo:chat-socket-open", onSharedOpen);
        };
        let stopped = false;
        let socket: WebSocket | null = null;
        let retry: number | null = null;
        const connect = () => {
            if (stopped) return;
            const url = new URL(WS_URL);
            url.searchParams.set("token", user.token);
            socket = new WebSocket(url.toString());
            socket.onopen = () => { refreshLoaded(); window.dispatchEvent(new Event("nachalo:chat-socket-open")); };
            socket.onmessage = event => {
                try {
                    const frame = JSON.parse(event.data) as { type?: string; payload?: unknown };
                    if (frame.type === "chat.message" || frame.type === "chat.system") receive(frame.payload);
                    if (frame.type === "presence.join" || frame.type === "presence.leave") window.dispatchEvent(new CustomEvent("nachalo:presence-event", { detail: frame }));
                } catch { /* Ignore unrelated game events. */ }
            };
            socket.onclose = () => { if (!stopped) retry = window.setTimeout(connect, 3000); };
        };
        connect();
        return () => {
            stopped = true;
            if (retry !== null) window.clearTimeout(retry);
            socket?.close(1000);
            window.removeEventListener("nachalo:chat-message", onSharedMessage);
            window.removeEventListener("nachalo:chat-socket-open", onSharedOpen);
        };
    }, [match, receive, refreshLoaded, user?.token, visible]);

    useEffect(() => {
        if (visible && user?.token && (channel !== "GROUP" || partyId) && !loadedRef.current[channel]) void loadHistory(channel, "initial");
    }, [channel, loadHistory, partyId, user?.token, visible]);

    useEffect(() => {
        if (visible && privateView && !privateLoaded.current.has(privateView.id)) void loadPrivateHistory(privateView.id, "initial");
    }, [visible, privateView, loadPrivateHistory]);

    useLayoutEffect(() => {
        const element = scrollRef.current;
        if (!element) return;
        if (restoreScroll.current) {
            element.scrollTop = element.scrollHeight - restoreScroll.current.height + restoreScroll.current.top;
            restoreScroll.current = null;
        } else if (stickToBottom.current) element.scrollTop = element.scrollHeight;
    }, [channel, expanded, lists, privateView, privateVersion]);

    const send = async () => {
        const body = draft.trim();
        if (!user?.token || sendingRef.current || (!privateView && channel === "GROUP" && !partyId)) return;
        if (!body || Array.from(body).length > 1000) { setError(text.invalid); return; }
        sendingRef.current = true;
        setSending(true);
        setError("");
        try {
            const sendChannel = privateView ? "PRIVATE" : channel;
            const response = await fetch(`${API_BASE}/game/chat/messages`, {
                method: "POST",
                headers: { Authorization: `Bearer ${user.token}`, "Content-Type": "application/json" },
                body: JSON.stringify({ channel: sendChannel, body, ...(privateView ? { conversationId: privateView.id } : {}) }),
            });
            if (!response.ok) {
                if (sendChannel === "GROUP" && response.status === 403) { setPartyId(null); partyRef.current = null; }
                setError(response.status === 429 ? text.limit : response.status === 403 ? text.denied : response.status === 400 ? text.invalid : text.failure);
                return;
            }
            const saved: unknown = await response.json();
            if (isChatMessage(saved)) receive(saved);
            setDraft(current => current === draft ? "" : current);
            stickToBottom.current = true;
        } catch { setError(text.failure); }
        finally { sendingRef.current = false; setSending(false); }
    };

    const rememberCaret = () => {
        const input = inputRef.current;
        if (input) caretRef.current = { start: input.selectionStart, end: input.selectionEnd };
    };

    const insertAtCaret = (value: string) => {
        const { start, end } = caretRef.current;
        const from = Math.min(start, draft.length);
        const to = Math.min(end, draft.length);
        pendingCaretRef.current = from + value.length;
        setDraft(`${draft.slice(0, from)}${value}${draft.slice(to)}`);
    };

    const mention = (nickname: string) => {
        const tag = `@${nickname} `;
        if (!draft.includes(tag)) {
            const from = Math.min(caretRef.current.start, draft.length);
            const prefix = from > 0 && !/\s$/.test(draft.slice(0, from)) ? " " : "";
            insertAtCaret(`${prefix}${tag}`);
        } else inputRef.current?.focus({ preventScroll: true });
    };

    const toggleEmoji = () => {
        if (emojiOpen) { setEmojiOpen(false); return; }
        rememberCaret();
        const rect = emojiButtonRef.current?.getBoundingClientRect();
        if (!rect) return;
        const width = Math.min(282, window.innerWidth - 16);
        const height = Math.min(226, window.innerHeight - 16);
        const left = Math.max(8, Math.min(rect.left, window.innerWidth - width - 8));
        const top = rect.top >= height + 12 ? rect.top - height - 6 : Math.max(8, Math.min(rect.bottom + 6, window.innerHeight - height - 8));
        setEmojiPosition({ left, top });
        setEmojiOpen(true);
    };

    const showOlder = () => {
        const element = scrollRef.current;
        if (element) restoreScroll.current = { height: element.scrollHeight, top: element.scrollTop };
        if (privateView) void loadPrivateHistory(privateView.id, "older");
        else void loadHistory(channel, "older");
    };

    if (!visible) return null;
    const rows = privateView ? privateMessages.current.get(privateView.id) ?? [] : lists[channel];
    const olderCursor = privateView ? privateCursors.current.get(privateView.id) : cursor[channel];
    const totalUnread = channels.reduce((sum, key) => sum + unread[key], 0) + conversations.reduce((sum, item) => sum + item.unreadCount, 0);
    const conversationName = (item: ChatConversation) => item.participants.filter(player => player.userId !== user?.id).map(player => player.name).join(", ") || `#${item.id}`;
    const leaveConversation = async () => {
        if (!privateView || privateView.kind !== "MULTI" || !user?.token) return;
        const id = privateView.id;
        try {
            const response = await fetch(`${API_BASE}/game/chat/conversations/${id}/leave`, { method: "POST", headers: { Authorization: `Bearer ${user.token}` } });
            if (!response.ok) { setError(text.failure); return; }
            setPrivateView(null);
            setConversations(current => current.filter(item => item.id !== id));
            setNotices(current => current.filter(item => item.conversationId !== id));
        } catch { setError(text.failure); }
    };
    const canSend = Boolean(user?.token && draft.trim() && Array.from(draft.trim()).length <= 1000 && !sending && (privateView || channel !== "GROUP" || partyId));
    return <section className={`${styles.chat} ${match ? styles.match : ""} ${expanded ? styles.open : ""}`} aria-label={text.title}>
        <button type="button" className={styles.header} aria-expanded={expanded} onClick={() => { setExpanded(value => !value); setEmojiOpen(false); if (privateView) void markRead(privateView.id); else setUnread(previous => ({ ...previous, [channel]: 0 })); stickToBottom.current = true; }}>
            <span>{text.title}</span><span className={styles.headerRight}>{totalUnread > 0 && <span className={styles.badge}>{totalUnread}</span>}{expanded ? "−" : "+"}</span>
        </button>
        {expanded && <div className={styles.content}>
            <div className={styles.tabs} role="tablist" aria-label={text.title}>
                {channels.map(key => <button key={key} type="button" role="tab" aria-selected={!privateView && channel === key} className={!privateView && channel === key ? styles.active : ""} onClick={() => { setEmojiOpen(false); setPrivateView(null); setChannel(key); setUnread(previous => ({ ...previous, [key]: 0 })); stickToBottom.current = true; setError(""); }}>
                    {{ MAIN: text.main, GROUP: text.group, TRADE: text.trade }[key]}{unread[key] > 0 && <span className={styles.tabBadge}>{unread[key]}</span>}
                </button>)}
            </div>
            <div className={styles.privateTabs} aria-label={text.private}>
                <button type="button" onClick={() => { setCreating(value => !value); setExpanded(true); }}>＋ Новая беседа</button>
                {conversations.map(item => <button type="button" key={item.id} className={privateView?.id === item.id ? styles.activePrivate : ""} onClick={() => openConversation(item)} title={item.participants.map(player => player.name).join(", ")}>{item.kind === "MULTI" ? "👥 " : "✉ "}{conversationName(item)}{item.unreadCount > 0 && <span className={styles.tabBadge}>{item.unreadCount}</span>}</button>)}
            </div>
            {creating && <div className={styles.conversationCreate}>
                <input type="search" value={playerSearch} onChange={event => setPlayerSearch(event.target.value)} placeholder="Найти игрока (в том числе не в сети)" aria-label="Найти игрока" />
                <div className={styles.playerChoices}>{playerResults.filter(player => player.userId !== user?.id && !selectedPlayers.some(item => item.userId === player.userId)).map(player => <button type="button" key={player.userId} onClick={() => addParticipant(player)}>{player.name}</button>)}</div>
                <div className={styles.selectedPlayers}>{selectedPlayers.map(player => <button type="button" key={player.userId} onClick={() => setSelectedPlayers(current => current.filter(item => item.userId !== player.userId))} title="Убрать">{player.name} ×</button>)}</div>
                <button type="button" disabled={!selectedPlayers.length} onClick={() => void createConversation(selectedPlayers)}>Создать</button>
            </div>}
            <div className={styles.chatColumns}>
              <div className={styles.messageColumn}>
                {privateView && <div className={styles.privateHeader}><span>{privateView.kind === "MULTI" ? "Беседа" : text.privateTo} <strong>{conversationName(privateView)}</strong><small title="Участники"> · {privateView.participants.map(player => player.name).join(", ")}</small></span>{privateView.kind === "MULTI" && <button type="button" onClick={() => void leaveConversation()}>Покинуть</button>}<button type="button" onClick={() => { setEmojiOpen(false); setPrivateView(null); setError(""); stickToBottom.current = true; }}>{text.backToChannels}</button></div>}
                <div className={styles.history} ref={scrollRef} onWheel={event => event.stopPropagation()} onScroll={event => { const target = event.currentTarget; const near = target.scrollHeight - target.scrollTop - target.clientHeight < 48; stickToBottom.current = near; setShowLatest(!near); }}>
                {olderCursor && <button type="button" className={styles.older} disabled={privateView ? privatePending.current.has(privateView.id) : loading === channel} onClick={showOlder}>{text.older}</button>}
                {notices.filter(item => item.conversationId !== privateView?.id).map(item => <button type="button" key={item.messageId} className={styles.privateNotice} onClick={() => { const target = conversations.find(conversation => conversation.id === item.conversationId); if (target) openConversation(target); else void refreshConversations().then(items => { const found = items?.find(conversation => conversation.id === item.conversationId); if (found) openConversation(found); }); }}><time dateTime={item.createdAt}>[{Number.isNaN(Date.parse(item.createdAt)) ? "--:--:--" : new Intl.DateTimeFormat(language, { hour: "2-digit", minute: "2-digit", second: "2-digit", hourCycle: "h23" }).format(new Date(item.createdAt))}]</time> Новое личное сообщение от {item.sender}!</button>)}
                {!privateView && channel === "GROUP" && !partyId ? <p className={styles.empty}>{partyUnavailable || !partyChecked ? text.unavailable : text.noParty}</p> : rows.length === 0 && loading !== channel ? <p className={styles.empty}>{text.empty}</p> : null}
                {(!privateView && channel === "GROUP" && !partyId ? [] : rows.filter(message => !mutedIds.includes(message.senderUserId))).map(message => <div className={`${styles.row} ${message.channel === "SYSTEM" ? styles.system : ""}`} key={message.id}>
                    <time dateTime={message.createdAt}>[{Number.isNaN(Date.parse(message.createdAt)) ? "--:--:--" : new Intl.DateTimeFormat(language, { hour: "2-digit", minute: "2-digit", second: "2-digit", hourCycle: "h23" }).format(new Date(message.createdAt))}]</time>{" "}
                    {message.channel === "SYSTEM" ? <strong>{message.senderNickname}:</strong> : <button type="button" className={styles.nickname} onClick={() => mention(message.senderNickname)}>{message.senderNickname}:</button>}{" "}
                    <ChatEmoteMessage body={message.body} animate={animateEmotes === true} />
                </div>)}
                </div>
                {showLatest && <button type="button" className={styles.latest} onClick={() => { stickToBottom.current = true; scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: "smooth" }); setShowLatest(false); }}>{text.latest}</button>}
                {error && <p className={styles.error} role="alert">{error}</p>}
                <div className={styles.composer}>
                    <textarea ref={inputRef} value={draft} rows={2} placeholder={privateView ? `${text.private}: ${conversationName(privateView)}` : text.placeholder} disabled={!privateView && channel === "GROUP" && !partyId} onChange={event => { setDraft(event.target.value); rememberCaret(); }} onSelect={rememberCaret} onKeyUp={rememberCaret} onClick={rememberCaret} onKeyDown={event => { if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing) { event.preventDefault(); void send(); } }} />
                    <button ref={emojiButtonRef} type="button" className={styles.emojiTrigger} aria-label={text.emoji} title={text.emoji} aria-expanded={emojiOpen} aria-controls="nachalo-chat-emoji-picker" disabled={!privateView && channel === "GROUP" && !partyId} onClick={toggleEmoji}>☺</button>
                    <button type="button" disabled={!canSend} onClick={() => void send()}>{sending ? text.sending : text.send}</button>
                </div>
              </div>
              {user && <OnlinePlayersPanel key={user.id} userId={user.id} token={user.token} language={language} mutedIds={mutedIds} onPrivate={openPrivate} onAddToConversation={addParticipant} onMutesChanged={changeMutes} />}
            </div>
        </div>}
        {emojiOpen && expanded && createPortal(<div id="nachalo-chat-emoji-picker" ref={emojiPanelRef} className={styles.emojiPicker} style={emojiPosition} aria-label={text.emoji} onWheel={event => event.stopPropagation()}>
            <div className={styles.pickerSections}>
                {(["classic", "fantasy", "emoji"] as const).map(section => <button key={section} type="button" className={pickerSection === section ? styles.pickerSectionActive : ""} aria-pressed={pickerSection === section} onClick={() => setPickerSection(section)}>{emoteText[section]}</button>)}
            </div>
            {pickerSection === "emoji" ? <>
                <div className={styles.emojiCategories}>
                    {emojiCategories.map(category => <button key={category.key} type="button" className={emojiCategory === category.key ? styles.emojiCategoryActive : ""} aria-label={text[category.key]} title={text[category.key]} aria-pressed={emojiCategory === category.key} onClick={() => setEmojiCategory(category.key)}>{category.icon}</button>)}
                </div>
                <div className={styles.emojiGrid} aria-label={text[emojiCategory]}>
                    {emojiCategories.find(category => category.key === emojiCategory)?.emojis.map(emoji => <button key={emoji} type="button" aria-label={`${text.emoji}: ${emoji}`} title={emoji} onClick={() => insertAtCaret(emoji)}>{emoji}</button>)}
                </div>
            </> : <div className={styles.emoteGrid} aria-label={emoteText[pickerSection]}>
                {chatEmotes.filter(emote => emote.category === pickerSection).length === 0 ? <span className={styles.noEmotes}>{emoteText.unavailable}</span> : chatEmotes.filter(emote => emote.category === pickerSection).map(emote => <button key={emote.id} type="button" title={`${emote.name} (${emote.code})`} aria-label={`${emote.name} (${emote.code})`} onClick={() => insertAtCaret(emote.code)}><EmoteImage emote={emote} animate={animateEmotes === true} /><span>{emote.name}</span></button>)}
            </div>}
            <label className={styles.animationSetting}><input type="checkbox" checked={animateEmotes === true} onChange={event => changeAnimation(event.target.checked)} />{emoteText.animation}</label>
        </div>, document.body)}
    </section>;
}
