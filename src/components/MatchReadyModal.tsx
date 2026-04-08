"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import styles from "../styles/ModeSelectionPage.module.css";

type Props = {
    instanceId: string;
    redirectAtMs: number | null;
    onClose: () => void;
    onJoin?: (instanceId: string) => void;
};

export default function MatchReadyModal({
    instanceId,
    redirectAtMs,
    onClose,
    onJoin,
}: Props) {
    const router = useRouter();
    const [secondsLeft, setSecondsLeft] = useState(() =>
        redirectAtMs
            ? Math.max(1, Math.ceil((redirectAtMs - Date.now()) / 1000))
            : 1,
    );

    useEffect(() => {
        if (!redirectAtMs) return;
        const update = () =>
            setSecondsLeft(
                Math.max(1, Math.ceil((redirectAtMs - Date.now()) / 1000)),
            );
        update();
        const t = setInterval(update, 250);
        const onKey = (e: KeyboardEvent) => {
            if (e.key === "Escape") onClose();
        };
        window.addEventListener("keydown", onKey);
        return () => {
            clearInterval(t);
            window.removeEventListener("keydown", onKey);
        };
    }, [redirectAtMs, onClose]);

    const joinNow = () => {
        if (onJoin) {
            onJoin(instanceId);
            return;
        }
        router.push(`/game?instance_id=${instanceId}`);
    };

    return (
        <div className={styles.modalOverlay} role="dialog" aria-modal="true">
            <div className={styles.matchReadyModal}>
                <button
                    className={styles.modalCloseButton}
                    aria-label="Close"
                    onClick={onClose}
                >
                    ✕
                </button>
                <div className={styles.matchReadyTitle}>Match Ready</div>
                <div className={styles.matchReadyText}>
                    Auto-start in {secondsLeft} sec.
                </div>
                <div
                    style={{
                        display: "flex",
                        gap: "0.5rem",
                        marginTop: "0.4rem",
                    }}
                >
                    <button className={styles.queueButton} onClick={joinNow}>
                        Start Match Now
                    </button>
                    <button
                        className={styles.partySecondaryButton}
                        onClick={onClose}
                    >
                        Close
                    </button>
                </div>
            </div>
        </div>
    );
}
