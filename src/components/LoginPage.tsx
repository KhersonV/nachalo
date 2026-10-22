"use client";

import { useRef, useState, type FormEvent } from "react";
import Link from "next/link";
import { AUTH_BASE } from "@/utils/serviceUrls";
import { useAuth } from "@/contexts/AuthContext";
import AuthShell from "./AuthShell";
import styles from "@/styles/AuthShell.module.css";

export default function LoginPage() {
  const { login } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState("");
  const submitting = useRef(false);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (submitting.current) return;
    submitting.current = true;
    setError("");
    setIsSubmitting(true);
    try {
      const response = await fetch(`${AUTH_BASE}/auth/login`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email, password }) });
      if (!response.ok) throw new Error((await response.text()) || "Login error");
      login(await response.json());
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Something went wrong");
    } finally {
      submitting.current = false;
      setIsSubmitting(false);
    }
  };

  return <AuthShell eyebrow="WELCOME BACK" title="Sign In" description="Your story is waiting where you left it.">
    <form className={styles.form} onSubmit={handleSubmit}>
      <div className={styles.field}><label htmlFor="login-email">Email</label><input id="login-email" name="email" className={styles.input} type="email" value={email} onChange={event => setEmail(event.target.value)} placeholder="you@example.com" autoComplete="email" required disabled={isSubmitting} /></div>
      <div className={styles.field}><label htmlFor="login-password">Password</label><div className={styles.passwordWrap}><input id="login-password" name="password" className={styles.input} type={showPassword ? "text" : "password"} value={password} onChange={event => setPassword(event.target.value)} placeholder="Enter your password" autoComplete="current-password" required disabled={isSubmitting} /><button type="button" className={styles.passwordToggle} aria-controls="login-password" aria-pressed={showPassword} onClick={() => setShowPassword(value => !value)}>{showPassword ? "Hide" : "Show"}</button></div></div>
      {error && <p className={styles.error} role="alert">{error}</p>}
      <button className={styles.submit} type="submit" disabled={isSubmitting} aria-busy={isSubmitting}>{isSubmitting ? "Signing in…" : "Enter the Game"}</button>
      <p className={styles.switch}>New to Nachalo? <Link href="/register">Create an account</Link></p>
    </form>
  </AuthShell>;
}
