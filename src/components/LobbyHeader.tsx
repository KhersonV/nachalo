"use client";

import React from "react";
import { useAuth } from "@/contexts/AuthContext";
import { API_BASE } from "@/utils/serviceUrls";
import { usePathname } from "next/navigation";
import Link from "next/link";
import styles from "../styles/LobbyHeader.module.css";
import LanguageSwitcher from "./LanguageSwitcher";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";

type NavItem = {
    id: string;
    label: string;
    iconSrc: string;
    href: string;
    isActive?: boolean;
};

function NavButton({ item, unread }: { item: NavItem; unread?: number }) {
    return (
        <div className={styles.navItemWrap}>
            <Link
                href={item.href}
                className={`${styles.navButton} ${item.isActive ? styles.navButtonActive : ""}`}
                aria-current={item.isActive ? "page" : undefined}
            >
                <img
                    src={item.iconSrc}
                    alt=""
                    className={styles.navIcon}
                    draggable={false}
                />
                <span className={styles.navHint}>{item.label}</span>{item.id === "market" && Boolean(unread) && <span className={styles.marketUnread} aria-label={`${unread} непрочитанных уведомлений`}>{unread}</span>}
            </Link>
        </div>
    );
}

export default function LobbyHeader() {
    const pathname = usePathname();
    const { language } = useGameLanguage();
    const { user } = useAuth();
    const [marketUnread, setMarketUnread] = React.useState(0);
    React.useEffect(() => { if (!user?.token) return; let active = true; const refresh = () => { void fetch(`${API_BASE}/game/market/notifications-count`, { headers: { Authorization: `Bearer ${user.token}` }, cache: "no-store" }).then(r => r.ok ? r.json() : null).then(data => { if (active && typeof data?.unread === "number") setMarketUnread(data.unread); }).catch(() => {}); }; refresh(); const timer = window.setInterval(refresh, 60000); return () => { active = false; window.clearInterval(timer); }; }, [user?.token, pathname]);

    const navItems: NavItem[] = [
        {
            id: "profile",
            label: "Profile",
            iconSrc: "/ui-icons/profile.png",
            isActive: pathname === "/profile",
            href: "/profile",
        },
        {
            id: "history",
            label: "History",
            iconSrc: "/ui-icons/statistick.png",
            isActive: pathname === "/matches" || pathname.startsWith("/matches/"),
            href: "/matches",
        },
        {
            id: "shop",
            label: "Shop",
            iconSrc: "/ui-icons/shop.png",
            isActive: pathname === "/shop",
            href: "/shop",
        },
        {
            id: "market",
            label: "Аукционный дом",
            iconSrc: "/ui-icons/market.svg",
            isActive: pathname === "/market",
            href: "/market",
        },
        {
            id: "base",
            label: "Base",
            iconSrc: "/ui-icons/base.png",
            isActive: pathname === "/base",
            href: "/base",
        },
        {
            id: "equipment",
            label: "Inventory",
            iconSrc: "/ui-icons/backpack.png",
            isActive: pathname === "/equipment",
            href: "/equipment",
        },
        {
            id: "professions",
            label: "Professions",
            iconSrc: "/ui-icons/professions.svg",
            isActive: pathname === "/professions",
            href: "/professions",
        },
        {
            id: "modes",
            label: "Modes",
            iconSrc: "/ui-icons/battle.png",
            isActive: pathname === "/mode",
            href: "/mode",
        },
    ];

    return (
        <header className={styles.header}>
            <div className={styles.headerTitleBlock}>
                <h1 className={styles.headerTitle}>{uiText(language, "Lobby")}</h1>
            </div>
            <nav className={styles.nav} aria-label="Lobby navigation">
                {navItems.map((item) => (
                    <NavButton key={item.id} item={{ ...item, label: uiText(language, item.label) }} unread={marketUnread} />
                ))}
            </nav>
            <LanguageSwitcher />
        </header>
    );
}
