//=====================================
// src/components/RegisterPage.tsx
//=====================================

"use client";

import React, { useState } from "react";
import { useRouter } from "next/navigation";
import styles from "../styles/RegisterPage.module.css";
import {
    characterArchetypes,
    type CharacterArchetype,
} from "../constants/characterArchetypes";

const AUTH_BASE = process.env.NEXT_PUBLIC_AUTH_BASE || "http://localhost:8000";

const RegisterPage = () => {
    const router = useRouter();
    const [email, setEmail] = useState("");
    const [password, setPassword] = useState("");
    const [name, setName] = useState("");
    const [characterType, setCharacterType] = useState<
        CharacterArchetype["id"]
    >(characterArchetypes[0].id);
    const [error, setError] = useState("");
    const [showPassword, setShowPassword] = useState(false);
    const [isSubmitting, setIsSubmitting] = useState(false);

    const selectedArchetype =
        characterArchetypes.find((item) => item.id === characterType) ??
        characterArchetypes[0];

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setError("");
        setIsSubmitting(true);

        try {
            const res = await fetch(`${AUTH_BASE}/auth/register`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({
                    email,
                    password,
                    name,
                    image: selectedArchetype.image,
                    characterType,
                }),
            });

            if (!res.ok) {
                const errorText = await res.text();
                throw new Error(errorText || "Registration error");
            }

            // You can navigate to the login page immediately or auto-login
            router.push("/login");
        } catch (err: any) {
            setError(err.message || "Something went wrong");
        } finally {
            setIsSubmitting(false);
        }
    };

    return (
        <section className={styles.page}>
            <div className={styles.glowA} aria-hidden="true" />
            <div className={styles.glowB} aria-hidden="true" />

            <div className={styles.card}>
                <p className={styles.kicker}>Create account</p>
                <h1 className={styles.title}>Sign Up</h1>
                <p className={styles.subtitle}>
                    Create a character and join the game.
                </p>

                <form onSubmit={handleSubmit} className={styles.form}>
                    <div className={styles.field}>
                        <label className={styles.label}>Email</label>
                        <input
                            className={styles.input}
                            type="email"
                            value={email}
                            onChange={(e) => setEmail(e.target.value)}
                            placeholder="you@example.com"
                            required
                        />
                    </div>

                    <div className={styles.field}>
                        <label className={styles.label}>Name</label>
                        <input
                            className={styles.input}
                            type="text"
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            placeholder="Your nickname"
                            required
                        />
                    </div>

                    <div className={styles.field}>
                        <label className={styles.label}>Password</label>
                        <div className={styles.passwordWrap}>
                            <input
                                className={styles.input}
                                type={showPassword ? "text" : "password"}
                                value={password}
                                onChange={(e) => setPassword(e.target.value)}
                                placeholder="Minimum 6 characters"
                                minLength={6}
                                required
                            />
                            <button
                                type="button"
                                className={styles.passwordToggle}
                                onClick={() => setShowPassword((v) => !v)}
                            >
                                {showPassword ? "Hide" : "Show"}
                            </button>
                        </div>
                    </div>

                    <div className={styles.field}>
                        <div className={styles.characterHeader}>
                            <label className={styles.label}>
                                Choose a character archetype
                            </label>
                            <p className={styles.characterHint}>
                                Class defines starting stats.
                            </p>
                            <div className={styles.statLegend}>
                                <p className={styles.statLegendTitle}>
                                    How to read stats:
                                </p>
                                <p className={styles.statLegendRow}>
                                    HP: character's health pool.
                                </p>
                                <p className={styles.statLegendRow}>
                                    Energy: resource for actions; restores each
                                    turn.
                                </p>
                                <p className={styles.statLegendRow}>
                                    Attack/Defense: outgoing and incoming damage
                                    in combat.
                                </p>
                                <p className={styles.statLegendRow}>
                                    Mobility/Agility: base movement-related
                                    stats of the archetype.
                                </p>
                                <p className={styles.statLegendRow}>
                                    Sight: how many tiles are visible around on
                                    the map.
                                </p>
                                <p className={styles.statLegendRow}>
                                    Combat type and range: melee or ranged and
                                    maximum attack distance.
                                </p>
                            </div>
                        </div>
                        <div className={styles.avatarGrid}>
                            {characterArchetypes.map((item) => (
                                <label
                                    key={item.id}
                                    className={`${styles.avatarOption} ${characterType === item.id ? styles.avatarOptionActive : ""}`}
                                >
                                    <input
                                        type="radio"
                                        name="characterType"
                                        value={item.id}
                                        checked={characterType === item.id}
                                        onChange={() =>
                                            setCharacterType(item.id)
                                        }
                                        className={styles.avatarRadio}
                                    />
                                    <img
                                        src={item.image}
                                        alt={item.name}
                                        className={styles.avatarImage}
                                    />
                                    <div className={styles.avatarContent}>
                                        <div className={styles.avatarTitleRow}>
                                            <strong>{item.name}</strong>
                                            <span className={styles.avatarTag}>
                                                {item.title}
                                            </span>
                                        </div>
                                        <p className={styles.avatarDescription}>
                                            {item.description}
                                        </p>
                                        <div className={styles.statsGrid}>
                                            {item.stats.map((stat) => (
                                                <span
                                                    key={`${item.id}-${stat.label}`}
                                                    className={styles.statChip}
                                                >
                                                    {stat.label}: {stat.value}
                                                </span>
                                            ))}
                                        </div>
                                    </div>
                                </label>
                            ))}
                        </div>
                    </div>

                    {error && <p className={styles.error}>{error}</p>}

                    <button
                        type="submit"
                        disabled={isSubmitting}
                        className={styles.submitButton}
                    >
                        {isSubmitting ? "Creating..." : "Register"}
                    </button>

                    <button
                        type="button"
                        className={styles.linkButton}
                        onClick={() => router.push("/login")}
                    >
                        Already have an account? Sign in
                    </button>
                </form>
            </div>
        </section>
    );
};

export default RegisterPage;
