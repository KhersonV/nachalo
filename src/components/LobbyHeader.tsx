"use client";

import React from "react";
import { useRouter } from "next/navigation";
import { usePathname } from "next/navigation";
import styles from "../styles/LobbyHeader.module.css";

type NavItem = {
    id: string;
    label: string;
    iconSrc: string;
    iconScale?: number;
    iconShiftY?: number;
    isActive?: boolean;
    onClick: () => void;
};

function NavButton({ item }: { item: NavItem }) {
    return (
        <div className={styles.navItemWrap}>
            <button
                type="button"
                className={`${styles.navButton} ${item.isActive ? styles.navButtonActive : ""}`}
                onClick={item.onClick}
                aria-label={item.label}
                title={item.label}
            >
                <img
                    src={item.iconSrc}
                    alt={item.label}
                    className={styles.navIcon}
                    draggable={false}
                    style={
                        {
                            "--icon-scale": item.iconScale ?? 1,
                            "--icon-shift-y": `${item.iconShiftY ?? 0}px`,
                        } as React.CSSProperties
                    }
                />
            </button>
            <span className={styles.navHint}>{item.label}</span>
        </div>
    );
}

export default function LobbyHeader() {
    const router = useRouter();
    const pathname = usePathname();

    const navItems: NavItem[] = [
        {
            id: "profile",
            label: "Профиль",
            iconSrc: "/ui-icons/profile.png",
            iconScale: 1.05,
            iconShiftY: -1,
            onClick: () => router.push("/profile"),
        },
        {
            id: "shop",
            label: "Магазин",
            iconSrc: "/ui-icons/shop.png",
            iconScale: 0.98,
            iconShiftY: 0,
            isActive: pathname === "/shop",
            onClick: () => router.push("/shop"),
        },
        {
            id: "base",
            label: "База",
            iconSrc: "/ui-icons/base.png",
            iconScale: 1.02,
            iconShiftY: 0,
            isActive: pathname === "/base",
            onClick: () => router.push("/base"),
        },
        {
            id: "equipment",
            label: "Рюкзак",
            iconSrc: "/ui-icons/backpack.png",
            iconScale: 0.99,
            iconShiftY: 1,
            onClick: () => router.push("/equipment"),
        },
    ];

    return (
        <header className={styles.header}>
            <div className={styles.headerTitleBlock}>
                <h1 className={styles.headerTitle}>Лобби</h1>
            </div>
            <nav className={styles.nav} aria-label="Навигация лобби">
                {navItems.map((item) => (
                    <NavButton key={item.id} item={item} />
                ))}
            </nav>
        </header>
    );
}
