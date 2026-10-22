"use client";

import React from "react";
import { usePathname } from "next/navigation";
import Link from "next/link";
import styles from "../styles/LobbyHeader.module.css";

type NavItem = {
    id: string;
    label: string;
    iconSrc: string;
    href: string;
    isActive?: boolean;
};

function NavButton({ item }: { item: NavItem }) {
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
                <span className={styles.navHint}>{item.label}</span>
            </Link>
        </div>
    );
}

export default function LobbyHeader() {
    const pathname = usePathname();

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
                <h1 className={styles.headerTitle}>Lobby</h1>
            </div>
            <nav className={styles.nav} aria-label="Lobby navigation">
                {navItems.map((item) => (
                    <NavButton key={item.id} item={item} />
                ))}
            </nav>
        </header>
    );
}
