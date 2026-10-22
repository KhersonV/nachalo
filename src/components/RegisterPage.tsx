"use client";

import UiText from "@/components/UiText";

import { useRef, useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { AUTH_BASE } from "@/utils/serviceUrls";
import { characterArchetypes, type CharacterArchetype } from "@/constants/characterArchetypes";
import { registrationPayload } from "@/features/auth/publicEntry";
import { getHeroPortrait } from "@/features/heroes/heroArtwork";
import AuthShell from "./AuthShell";
import styles from "@/styles/AuthShell.module.css";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";

export default function RegisterPage() {
  const { language } = useGameLanguage();
  const t = (source: string) => uiText(language, source);
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [characterType, setCharacterType] = useState<CharacterArchetype["id"]>(characterArchetypes[0].id);
  const [showPassword, setShowPassword] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState("");
  const submitting = useRef(false);
  const selectedArchetype = characterArchetypes.find(item => item.id === characterType) ?? characterArchetypes[0];

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (submitting.current) return;
    submitting.current = true;
    setError("");
    setIsSubmitting(true);
    try {
      const response = await fetch(`${AUTH_BASE}/auth/register`, {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify(registrationPayload({ email, password, name }, selectedArchetype)),
      });
      if (!response.ok) throw new Error((await response.text()) || t("Registration error"));
      router.push("/login");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("Something went wrong"));
    } finally {
      submitting.current = false;
      setIsSubmitting(false);
    }
  };

  return <AuthShell eyebrow={t("BEGIN YOUR STORY")} title={t("Create Account")} description={t("A name, a path, and a world waiting beyond the gate.")} wide>
    <form className={styles.form} onSubmit={handleSubmit}>
      {error && <p className={styles.error} role="alert">{error}</p>}
      <div className={styles.field}><label htmlFor="register-email"><UiText text="Email" /></label><input id="register-email" name="email" className={styles.input} type="email" value={email} onChange={event => setEmail(event.target.value)} placeholder="you@example.com" autoComplete="email" required disabled={isSubmitting} /></div>
      <div className={styles.field}><label htmlFor="register-name"><UiText text="Name" /></label><input id="register-name" name="name" className={styles.input} type="text" value={name} onChange={event => setName(event.target.value)} placeholder={t("Your nickname")} autoComplete="nickname" required disabled={isSubmitting} /></div>
      <div className={styles.field}><label htmlFor="register-password"><UiText text="Password" /></label><div className={styles.passwordWrap}><input id="register-password" name="password" className={styles.input} type={showPassword ? "text" : "password"} value={password} onChange={event => setPassword(event.target.value)} placeholder={t("At least 6 characters")} autoComplete="new-password" minLength={6} required disabled={isSubmitting} /><button type="button" className={styles.passwordToggle} aria-controls="register-password" aria-pressed={showPassword} onClick={() => setShowPassword(value => !value)}>{t(showPassword ? "Hide" : "Show")}</button></div></div>
      <fieldset className={styles.archetype}>
        <legend><UiText text="Choose Your Starting Hero" /></legend>
        <p className={styles.archetypeHint}><UiText text="Select the hero who will begin your journey." /></p>
        <div className={styles.archetypeGrid}>
          {characterArchetypes.map(item => <label key={item.id} className={`${styles.archetypeOption} ${characterType === item.id ? styles.archetypeSelected : ""}`}>
            <input type="radio" name="characterType" value={item.id} checked={characterType === item.id} onChange={() => setCharacterType(item.id)} disabled={isSubmitting} />
            <img src={getHeroPortrait(item.id)} alt="" />
            <span><strong>{t(item.name)}</strong><small>{t(item.title)}</small></span>
          </label>)}
        </div>
      </fieldset>
      <button className={styles.submit} type="submit" disabled={isSubmitting} aria-busy={isSubmitting}>{t(isSubmitting ? "Creating account…" : "Create Account")}</button>
      <p className={styles.switch}><UiText text="Already have an account?" /> <Link href="/login"><UiText text="Sign In" /></Link></p>
    </form>
  </AuthShell>;
}
