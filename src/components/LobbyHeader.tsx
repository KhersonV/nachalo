"use client";

import React from "react";
import { useAuth } from "@/contexts/AuthContext";
import { API_BASE } from "@/utils/serviceUrls";
import { usePathname, useSearchParams } from "next/navigation";
import Link from "next/link";
import styles from "../styles/LobbyHeader.module.css";
import LanguageSwitcher from "./LanguageSwitcher";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";
import { lobbyNavigationGroups, lobbyNavigationItems, lobbyItemIsActive, type LobbyGroupId, type LobbyIcon, type LobbyNavigationItem } from "@/config/lobbyNavigation";

// Match the stroke SVG treatment already used by the game's action controls.
const iconPaths: Record<LobbyIcon, string> = {
    swords: "m4 3 4 1 11 13-3 3L4 7ZM20 3l-4 1-4 5m-3 4-4 5m-2-2 5 5m7-6 6 5M3 21l3-3",
    castle: "M3 21V8h3V4h3v4h6V4h3v4h3v13ZM9 21v-6h6v6M6 11h2m8 0h2",
    hero: "M8 7a4 4 0 1 0 8 0 4 4 0 1 0-8 0ZM4 21v-2a8 8 0 0 1 16 0v2",
    trade: "M12 3v18M6 21h12M3 7h18M6 7l-4 8h8Zm12 0-4 8h8Z",
    book: "M12 5C8 2 4 3 2 4v15c4-2 7-1 10 1 3-2 6-3 10-1V4c-2-1-6-2-10 1Zm0 0v15",
    more: "M4 12h1m6 0h1m6 0h1",
    backpack: "M8 7V5a4 4 0 0 1 8 0v2M7 7h10a3 3 0 0 1 3 3v11H4V10a3 3 0 0 1 3-3ZM8 13h8v5H8Zm0-3h8",
    tools: "m4 21 10-11M5 4c6-3 12 1 15 7l-5-2-3-4-7-1Z",
    shop: "M4 10v11h16V10M2 10l3-7h14l3 7ZM9 21v-7h6v7",
    auction: "m5 8 7-6 5 6-7 6Zm8 3 8 9M3 21h10M5 18h6v3",
    exchange: "M3 7h17m-4-4 4 4-4 4M21 17H4m4-4-4 4 4 4",
    profile: "M8 7a4 4 0 1 0 8 0 4 4 0 1 0-8 0ZM4 21v-2a8 8 0 0 1 16 0v2",
    history: "M3 10a9 9 0 1 1 0 5M3 3v7h7M12 7v5l4 2",
    logout: "M10 3H4v18h6M9 12h13m-5-5 5 5-5 5",
};
function NavIcon({ name }: { name: LobbyIcon }) {
    return <svg className={styles.navIcon} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d={iconPaths[name]} /></svg>;
}

export default function LobbyHeader() {
    const pathname = usePathname();
    const searchParams = useSearchParams();
    const routeKey = `${pathname}?${searchParams.toString()}`;
    const { language } = useGameLanguage();
    const { user, logout } = useAuth();
    const t = (source: string) => uiText(language, source);
    const [marketUnread, setMarketUnread] = React.useState(0);
    const [open, setOpen] = React.useState<LobbyGroupId | null>(null);
    const [width, setWidth] = React.useState(1200);
    const root = React.useRef<HTMLElement>(null);
    const triggers = React.useRef<Partial<Record<LobbyGroupId, HTMLButtonElement | null>>>({});
    const menus = React.useRef<Partial<Record<LobbyGroupId, HTMLDivElement | null>>>({});
    const menuId = React.useId();
    React.useEffect(() => { if (!user?.token) return; let active = true; const refresh = () => { void fetch(`${API_BASE}/game/market/notifications-count`, { headers: { Authorization: `Bearer ${user.token}` }, cache: "no-store" }).then(r => r.ok ? r.json() : null).then(data => { if (active && typeof data?.unread === "number") setMarketUnread(data.unread); }).catch(() => {}); }; refresh(); const timer = window.setInterval(refresh, 60000); return () => { active = false; window.clearInterval(timer); }; }, [user?.token, pathname]);
    React.useEffect(() => { setOpen(null); }, [routeKey]);
    React.useEffect(() => {
        const header = root.current;
        if (!header) return;
        const resize = () => setWidth(header.getBoundingClientRect().width);
        const observer = new ResizeObserver(resize); observer.observe(header); resize();
        return () => observer.disconnect();
    }, []);
    React.useEffect(() => { setOpen(null); }, [width]);
    React.useEffect(() => {
        if (!open) return;
        const outside = (event: Event) => { if (event.target instanceof Node && !triggers.current[open]?.parentElement?.contains(event.target)) setOpen(null); };
        const escape = (event: KeyboardEvent) => {
            if (event.key === "Escape") { event.preventDefault(); setOpen(null); triggers.current[open]?.focus(); }
        };
        document.addEventListener("pointerdown", outside);
        document.addEventListener("focusin", outside);
        document.addEventListener("keydown", escape);
        return () => { document.removeEventListener("pointerdown", outside); document.removeEventListener("focusin", outside); document.removeEventListener("keydown", escape); };
    }, [open]);
    React.useLayoutEffect(() => {
        if (!open) return;
        const menu = menus.current[open];
        if (!menu) return;
        menu.style.setProperty("--lobby-menu-shift", "0px");
        const rect = menu.getBoundingClientRect();
        const shift = rect.left < 8 ? 8 - rect.left : rect.right > window.innerWidth - 8 ? window.innerWidth - 8 - rect.right : 0;
        menu.style.setProperty("--lobby-menu-shift", `${shift}px`);
    }, [open, width, language]);
    const available = lobbyNavigationItems.filter(item => !item.requiresUser || !!user);
    const folded = new Set(lobbyNavigationGroups.filter(group => group.foldBelow !== undefined && width < group.foldBelow).map(group => group.id));
    const foldBase = width < 540;
    const groupItems = (id: LobbyGroupId) => available.filter(item => item.group === id).sort((a, b) => a.order - b.order);
    const isActive = (item: LobbyNavigationItem) => lobbyItemIsActive(item, pathname);
    const isCurrentPage = (item: LobbyNavigationItem) => isActive(item) && (!item.href?.includes("?mode=") || (searchParams.get("mode") || "auction") === new URLSearchParams(item.href.split("?")[1]).get("mode"));
    const unreadBadge = <span className={styles.marketUnread} aria-label={`${t("Unread notifications")}: ${marketUnread}`}>{marketUnread}</span>;
    const focusMenu = (id: LobbyGroupId, last = false) => {
        setOpen(id);
        window.requestAnimationFrame(() => {
            const items = menus.current[id]?.querySelectorAll<HTMLElement>('[role="menuitem"]');
            if (items?.length) items[last ? items.length - 1 : 0].focus();
        });
    };
    const renderItem = (item: LobbyNavigationItem) => {
        const content = <><NavIcon name={item.icon} /><span>{t(item.label)}</span>{item.id === "auction" && marketUnread > 0 && unreadBadge}</>;
        const className = `${styles.menuItem} ${isCurrentPage(item) ? styles.menuItemActive : ""}`;
        return item.href ? <Link key={item.id} role="menuitem" href={item.href} className={className} aria-current={isCurrentPage(item) ? "page" : undefined} onClick={() => setOpen(null)}>{content}</Link>
            : <button key={item.id} type="button" role="menuitem" className={`${className} ${styles.logout}`} onClick={() => { setOpen(null); if (item.action === "logout") logout(); }}>{content}</button>;
    };
    return <header className={styles.header} ref={root}>
        <span className={styles.brand}>NACHALO</span>
        <nav className={styles.nav} aria-label={t("Lobby navigation")}>
            {available.filter(item => item.group === "primary" && !(foldBase && item.id === "base")).sort((a, b) => a.order - b.order).map(item => item.href && <Link key={item.id} href={item.href} className={`${styles.navButton} ${item.id === "play" ? styles.play : ""} ${isActive(item) ? styles.navButtonActive : ""}`} aria-current={isActive(item) ? "page" : undefined} onClick={() => setOpen(null)}><NavIcon name={item.icon} /><span>{t(item.label)}</span></Link>)}
            {lobbyNavigationGroups.filter(group => !folded.has(group.id)).sort((a, b) => a.order - b.order).map(group => {
                const children = groupItems(group.id);
                const overflowItems = available.filter(item => item.group !== "primary" && folded.has(item.group) || foldBase && item.id === "base");
                const active = [...children, ...(group.id === "more" ? overflowItems : [])].some(isActive);
                const expanded = open === group.id;
                return <div className={styles.navGroup} key={group.id}>
                    <button type="button" ref={element => { triggers.current[group.id] = element; }} className={`${styles.navButton} ${active ? styles.navButtonActive : ""}`} aria-haspopup="menu" aria-controls={`${menuId}-${group.id}`} aria-expanded={expanded} onClick={() => setOpen(expanded ? null : group.id)} onKeyDown={event => { if (event.key === "ArrowDown" || event.key === "ArrowUp") { event.preventDefault(); focusMenu(group.id, event.key === "ArrowUp"); } }}>
                        <NavIcon name={group.icon} /><span>{t(group.label)}</span>{group.id === "trade" && marketUnread > 0 && unreadBadge}{group.id === "more" && folded.has("trade") && marketUnread > 0 && unreadBadge}<svg className={styles.chevron} viewBox="0 0 12 12" aria-hidden="true"><path d="m3 4 3 3 3-3" /></svg>
                    </button>
                    <div hidden={!expanded} id={`${menuId}-${group.id}`} ref={element => { menus.current[group.id] = element; }} role="menu" aria-label={t(group.label)} className={`${styles.dropdown} ${group.id === "more" ? styles.dropdownEnd : ""}`} onKeyDown={event => {
                        const items = Array.from(event.currentTarget.querySelectorAll<HTMLElement>('[role="menuitem"]'));
                        const index = items.findIndex(item => item === document.activeElement);
                        if (["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) {
                            event.preventDefault();
                            const next = event.key === "Home" ? 0 : event.key === "End" ? items.length - 1 : (index + (event.key === "ArrowDown" ? 1 : -1) + items.length) % items.length;
                            items[next]?.focus();
                        } else if (event.key === "Tab") {
                            event.preventDefault(); setOpen(null);
                            const controls = Array.from(root.current?.querySelectorAll<HTMLElement>('nav > a, nav > div > button, select') ?? []);
                            const trigger = triggers.current[group.id];
                            const next = event.shiftKey ? trigger : controls[controls.findIndex(item => item === trigger) + 1];
                            (next || trigger)?.focus();
                        }
                    }}>
                        {children.filter(item => item.action !== "logout").map(renderItem)}
                        {group.id === "more" && <>
                            {foldBase && available.filter(item => item.id === "base").map(renderItem)}
                            {lobbyNavigationGroups.filter(item => folded.has(item.id)).map(section => <React.Fragment key={section.id}><div className={styles.menuHeading} role="presentation">{t(section.label)}</div>{groupItems(section.id).map(renderItem)}</React.Fragment>)}
                        </>}
                        {children.filter(item => item.action === "logout").map(renderItem)}
                    </div>
                </div>;
            })}
        </nav>
        <LanguageSwitcher className={styles.language} />
    </header>;
}
