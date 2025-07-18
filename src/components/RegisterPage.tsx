//=====================================
// src/components/RegisterPage.tsx
//=====================================

"use client";

import React, { useState } from "react";
import { useRouter } from "next/navigation";

// Массив доступных картинок
const avatarOptions = [
  "/Character_1.webp",
  "/Character_2.webp",
  "/player-1.webp",
  "/player-2.webp",
];

const RegisterPage = () => {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [name, setName] = useState("");
  const [image, setImage] = useState(avatarOptions[0]);
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    try {
      const res = await fetch("http://localhost:8000/auth/register", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ email, password, name, image }),
      });

      if (!res.ok) {
        const errorText = await res.text();
        throw new Error(errorText || "Ошибка регистрации");
      }

      // Можно сразу перейти на страницу логина или автоматически войти
      router.push("/login");
    } catch (err: any) {
      setError(err.message || "Что-то пошло не так");
    }
  };

  return (
    <div>
      <h1>Регистрация</h1>
      <form onSubmit={handleSubmit}>
        <div>
          <label>Email:</label>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </div>
        <div>
          <label>Имя:</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
        </div>
        <div>
          <label>Пароль:</label>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </div>
        <div>
          <label>Выберите аватар:</label>
          <div style={{ display: "flex", gap: 16, marginTop: 8 }}>
            {avatarOptions.map((src) => (
              <label key={src} style={{ cursor: "pointer" }}>
                <input
                  type="radio"
                  name="avatar"
                  value={src}
                  checked={image === src}
                  onChange={() => setImage(src)}
                  style={{ display: "none" }}
                />
                <img
                  src={src}
                  alt="avatar"
                  style={{
                    width: 60,
                    height: 60,
                    border:
                      image === src
                        ? "2px solid #1976d2"
                        : "2px solid transparent",
                    borderRadius: 10,
                    boxShadow: image === src ? "0 0 4px #1976d2" : "none",
                    transition: "border 0.2s, box-shadow 0.2s",
                  }}
                />
              </label>
            ))}
          </div>
        </div>
        {error && <p style={{ color: "red" }}>{error}</p>}
        <button type="submit">Зарегистрироваться</button>
      </form>
    </div>
  );
};

export default RegisterPage;
