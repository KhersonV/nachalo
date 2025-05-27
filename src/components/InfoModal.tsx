
//==============================
// src/components/InfoModal.tsx
//==============================

"use client";

import React, { useState, useEffect } from "react";
import { createPortal } from "react-dom";
import styles from "../styles/InfoModal.module.css";

// общий тип для всех игровых объектов
export type GameObject =
  | { type: "monster"; name: string; hp?: number; health?: number; aggressive: boolean; attack: number; defense: number }
  | { type: "resource"; name: string; description: string; effects: string[] }
  | { type: "player"; name: string; hp?: number; health?: number; defense: number; attack: number }
  | { type: "portal"; requirement: string }
  | { type: "cell"; x: number; y: number }
  | { type: "empty"; x: number; y: number }
  | { type: "wall"; x: number; y: number };

interface InfoModalProps {
  object: GameObject | null;
  onClose: () => void;
}

const getHP = (obj: GameObject): number => {
  if ("hp" in obj && obj.hp !== undefined) return obj.hp;
  if ("health" in obj && obj.health !== undefined) return obj.health;
  return 0;
};



export const InfoModal: React.FC<InfoModalProps> = ({ object, onClose }) => {
  if (!object) return null;

  const modalRoot = typeof document !== "undefined" && document.getElementById("modal-root");
  if (!modalRoot) return null;

  // Закрытие по Esc
  useEffect(() => {
    const handler = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [onClose]);

  // выбираем заголовок
  let title: string;
  if ("name" in object) title = object.name;
  else if (object.type === "portal") title = "Portal";
  else if (object.type === "cell") title = "Cell";
  else if (object.type === "empty") title = "Empty";
  else title = "Wall";

  // контент в зависимости от типа
  const renderContent = () => {
    switch (object.type) {
      case "resource":
        return (
          <>
            <div>{object.description}</div>
            <ul>{object.effects.map((e, i) => <li key={i}>{e}</li>)}</ul>
          </>
        );
      case "monster":
        return (
          <>
            <div><strong>HP:</strong> {getHP(object)}</div>
            <div><strong>Aggressive:</strong> {object.aggressive ? "Yes" : "No"}</div>
            <div><strong>Attack:</strong> {object.attack}</div>
            <div><strong>Defense:</strong> {object.defense}</div>
          </>
        );
      case "player":
        return (
          <>
            <div><strong>HP:</strong> {getHP(object)}</div>
            <div><strong>Defense:</strong> {object.defense}</div>
            <div><strong>Attack:</strong> {object.attack}</div>
          </>
        );
      case "portal":
        return <div><strong>Requires:</strong> {object.requirement}</div>;
      case "cell":
      case "empty":
      case "wall":
        return <div><strong>Coords:</strong> {object.x}, {object.y}</div>;
    }
  };

  return createPortal(
    <div className={styles.overlay} onClick={onClose}>
      <div className={styles.modal} onClick={e => e.stopPropagation()}>
        <header className={styles.header}>
          <h3 className={styles.title}>{title}</h3>
          <button onClick={onClose} className={styles.close}>&times;</button>
        </header>
        <section className={styles.content}>{renderContent()}</section>
      </div>
    </div>,
    modalRoot
  );
};

// хук для управления состоянием модалки
export function useInfoModal() {
  const [obj, setObj] = useState<GameObject | null>(null);
  return {
    open: (o: GameObject) => setObj(o),
    close: () => setObj(null),
    Modal: () => <InfoModal object={obj} onClose={() => setObj(null)} />,
  };
}
