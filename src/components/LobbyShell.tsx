"use client";

import type { ReactNode } from "react";
import { usePathname, useSearchParams } from "next/navigation";
import LobbyHeader from "./LobbyHeader";
import styles from "../styles/LobbyShell.module.css";

export default function LobbyShell({ children }: { children: ReactNode }) {
    const pathname = usePathname();
    const searchParams = useSearchParams();
    const viewedUser = Number.parseInt(searchParams.get("view") || "", 10);
    const externalProfile = pathname === "/profile" && Number.isFinite(viewedUser) && viewedUser > 0;

    if (externalProfile) return <div className={styles.shell}><div className={styles.frame}>{children}</div></div>;

    return (
        <div className={`${styles.shell} ${pathname === "/mode" ? styles.modeShell : ""}`}>
            <div className={`${styles.frame} ${pathname === "/mode" ? styles.wideFrame : ""}`}>
                <LobbyHeader />
                <div className={styles.content}>{children}</div>
            </div>
        </div>
    );
}
