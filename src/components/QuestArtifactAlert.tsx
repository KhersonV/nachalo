//==================================
// src/components/QuestArtifactAlert.tsx
//==================================

"use client";

import React, { useEffect } from "react";
import { createPortal } from "react-dom";
import styles from "../styles/QuestArtifactAlert.module.css";

interface QuestArtifactAlertProps {
    artifactId: number;
    name: string;
    image: string;
    description: string;
    badgeText?: string;
    hintText?: string;
    confirmLabel?: string;
    onClose: () => void;
}

const QuestArtifactAlert: React.FC<QuestArtifactAlertProps> = ({
    name,
    image,
    description,
    badgeText = "Quest",
    hintText = "Find this artifact to exit the level through the portal.",
    confirmLabel = "Got it",
    onClose,
}) => {
    useEffect(() => {
        const handler = (e: KeyboardEvent) => e.key === "Escape" && onClose();
        document.addEventListener("keydown", handler);
        return () => document.removeEventListener("keydown", handler);
    }, [onClose]);

    const modalRoot =
        typeof document !== "undefined" &&
        document.getElementById("modal-root");
    if (!modalRoot) return null;

    const imageUrl = image
        ? image.startsWith("http") || image.startsWith("/")
            ? image
            : `/artifacts/${image}`
        : null;

    return createPortal(
        <div className={styles.overlay} onClick={onClose}>
            <div
                className={styles.alert}
                onClick={(e) => e.stopPropagation()}
                role="dialog"
                aria-modal="true"
                aria-labelledby="quest-alert-title"
            >
                <div className={styles.header}>
                    <span className={styles.badge}>{badgeText}</span>
                    <button
                        className={styles.close}
                        onClick={onClose}
                        aria-label="Close"
                    >
                        ✕
                    </button>
                </div>

                <div className={styles.body}>
                    {imageUrl && (
                        <img
                            src={imageUrl}
                            alt={name}
                            className={styles.image}
                        />
                    )}
                    <h2 id="quest-alert-title" className={styles.title}>
                        {name}
                    </h2>
                    {description && (
                        <p className={styles.description}>{description}</p>
                    )}
                    <p className={styles.hint}>{hintText}</p>
                </div>

                <div className={styles.footer}>
                    <button className={styles.confirmButton} onClick={onClose}>
                        {confirmLabel}
                    </button>
                </div>
            </div>
        </div>,
        modalRoot,
    );
};

export default QuestArtifactAlert;
