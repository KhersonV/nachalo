"use client";

import React from "react";
import { useRouter } from "next/navigation";
import RequireAuth from "../../components/RequireAuth";
import LobbyHeader from "../../components/LobbyHeader";

export default function EquipmentPage() {
    const router = useRouter();

    return (
        <RequireAuth>
            <main
                style={{
                    padding: "var(--lobby-page-padding)",
                    color: "#eefbff",
                    minHeight: "100vh",
                    background:
                        "radial-gradient(circle at 0% 0%, rgba(126,228,255,0.1), transparent 45%), linear-gradient(rgba(10, 30, 48, 0.95), rgba(5, 13, 22, 0.92))",
                }}
            >
                <div
                    style={{
                        maxWidth: "var(--lobby-page-max-width)",
                        margin: "0 auto",
                    }}
                >
                    <LobbyHeader />
                </div>
                <div
                    style={{
                        maxWidth: "var(--lobby-page-max-width)",
                        margin: "0 auto",
                        border: "1px solid rgba(126,228,255,0.25)",
                        borderRadius: 14,
                        padding: "1rem",
                        background:
                            "linear-gradient(rgba(16, 42, 62, 0.82), rgba(6, 19, 31, 0.86))",
                    }}
                >
                    <h1 style={{ margin: 0, fontSize: "1.35rem" }}>
                        Экипировка персонажа
                    </h1>
                    <p style={{ marginTop: "0.5rem", opacity: 0.86 }}>
                        Базовая страница уже добавлена. Здесь будет система
                        слотов и надевания артефактов.
                    </p>

                    <section
                        style={{
                            marginTop: "1rem",
                            display: "grid",
                            gridTemplateColumns:
                                "repeat(auto-fit, minmax(180px, 1fr))",
                            gap: "0.75rem",
                        }}
                    >
                        {[
                            "Голова",
                            "Тело",
                            "Руки",
                            "Ноги",
                            "Артефакт 1",
                            "Артефакт 2",
                        ].map((slot) => (
                            <div
                                key={slot}
                                style={{
                                    border: "1px solid rgba(255,255,255,0.18)",
                                    borderRadius: 10,
                                    padding: "0.8rem",
                                    background: "rgba(23, 54, 77, 0.66)",
                                    minHeight: 86,
                                }}
                            >
                                <strong>{slot}</strong>
                                <div
                                    style={{
                                        marginTop: "0.45rem",
                                        opacity: 0.7,
                                    }}
                                >
                                    Пусто
                                </div>
                            </div>
                        ))}
                    </section>

                    <div
                        style={{
                            marginTop: "1rem",
                            display: "flex",
                            gap: "0.6rem",
                            flexWrap: "wrap",
                        }}
                    >
                        <button
                            type="button"
                            onClick={() => router.push("/mode")}
                            style={{
                                border: "none",
                                borderRadius: 8,
                                padding: "0.6rem 0.85rem",
                                color: "#072736",
                                fontWeight: 700,
                                cursor: "pointer",
                                background:
                                    "linear-gradient(rgba(126,228,255,0.92), rgba(18,162,205,0.9))",
                            }}
                        >
                            Вернуться в лобби
                        </button>
                    </div>
                </div>
            </main>
        </RequireAuth>
    );
}
