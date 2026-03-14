//==================================
// src/components/MapWithCamera.tsx
//==================================

"use client";

import React from "react";
import { useSelector } from "react-redux";
import { shallowEqual } from "react-redux";
import type { RootState } from "@/store";
import Map from "./Map";
import type { PlayerState } from "../types/GameTypes";
import { useInfoModal } from "./InfoModal";
import { cellToGameObject, playerToGameObject } from "../utils/toGameObject";
import { useCombatFloaters } from "../hooks/useCombatFloaters";
import styles from "../styles/Map.module.css";

interface MapWithCameraProps {
    tileSize: number;
    viewportWidth: number;
    viewportHeight: number;
    myPlayer: PlayerState;
}

export default function MapWithCamera({
    tileSize: inputTileSize,
    viewportWidth,
    viewportHeight,
    myPlayer,
}: MapWithCameraProps) {
    // console.log("🔄 MapWithCamera rerendered");

    const { grid, mapWidth, mapHeight, players, active_user } = useSelector(
        (state: RootState) => ({
            grid: state.game.grid,
            mapWidth: state.game.mapWidth,
            mapHeight: state.game.mapHeight,
            players: state.game.players,
            active_user: state.game.active_user,
        }),
        shallowEqual,
    );

    const playerPosition = myPlayer?.position || { x: 0, y: 0 };
    const visionRange = myPlayer?.vision ?? 3;
    const tileSize = Number(inputTileSize) || 60;
    const safeMapWidth = Number(mapWidth) || 15;
    const safeMapHeight = Number(mapHeight) || 15;
    const gap = 1;

    let offsetX =
        viewportWidth / 2 -
        (playerPosition.x * (tileSize + gap) + tileSize / 2);
    let offsetY =
        viewportHeight / 2 -
        (playerPosition.y * (tileSize + gap) + tileSize / 2);

    const totalWidth = safeMapWidth * tileSize + (safeMapWidth - 1) * gap;
    const totalHeight = safeMapHeight * tileSize + (safeMapHeight - 1) * gap;

    offsetX = Math.min(0, Math.max(viewportWidth - totalWidth, offsetX));
    offsetY = Math.min(0, Math.max(viewportHeight - totalHeight, offsetY));

    const playerImageOffsetX = 2;
    const playerImageOffsetY = 2;

    const { open, Modal } = useInfoModal();
    const { floaters, flashes } = useCombatFloaters(players, grid);

    return (
        <div
            style={{
                width: viewportWidth,
                height: viewportHeight,
                overflow: "hidden",
                position: "relative",
                border: "2px solid #000",
            }}
        >
            <div
                style={{
                    position: "absolute",
                    top: offsetY,
                    left: offsetX,
                    transition: "top 0.3s, left 0.3s",
                }}
            >
                <Map
                    grid={grid}
                    mapWidth={safeMapWidth}
                    mapHeight={safeMapHeight}
                    tileSize={tileSize}
                    gap={gap}
                    visionRange={visionRange}
                    playerPosition={playerPosition}
                    onCellClick={(cell) => {
                        console.log("Клик по клетке для перемещения:", cell);
                    }}
                />

                {players.map((player) => {
                    const dx = Math.abs(player.position.x - playerPosition.x);
                    const dy = Math.abs(player.position.y - playerPosition.y);
                    const playerVisible =
                        dx <= visionRange && dy <= visionRange;

                    // Игрок вне зоны видимости — не рендерим совсем
                    if (!playerVisible) return null;

                    return (
                        <img
                            key={player.user_id}
                            src={player.image}
                            alt={player.name}
                            title={player.name}
                            onClick={() => open(playerToGameObject(player))}
                            style={{
                                position: "absolute",
                                left:
                                    player.position.x * (tileSize + gap) +
                                    playerImageOffsetX,
                                top:
                                    player.position.y * (tileSize + gap) +
                                    playerImageOffsetY,
                                width: tileSize,
                                height: tileSize,
                                border:
                                    player.user_id === active_user
                                        ? "2px solid gold"
                                        : "none",
                                boxSizing: "border-box",
                                zIndex: 10,
                            }}
                        />
                    );
                })}

                {/* ---- Боевые вспышки на клетках ---- */}
                {flashes.map((fl) => (
                    <div
                        key={fl.id}
                        className={styles.combatFlash}
                        style={{
                            left: fl.x * (tileSize + gap),
                            top: fl.y * (tileSize + gap),
                            width: tileSize,
                            height: tileSize,
                        }}
                    />
                ))}

                {/* ---- Всплывающие числа урона / лечения ---- */}
                {floaters.map((fl) => (
                    <div
                        key={fl.id}
                        className={`${styles.combatFloater} ${
                            fl.isHeal
                                ? styles.combatFloaterHeal
                                : styles.combatFloaterDamage
                        }`}
                        style={{
                            left: fl.x * (tileSize + gap) + tileSize / 2,
                            top: fl.y * (tileSize + gap) - 2,
                        }}
                    >
                        {fl.isHeal ? `+${fl.value}` : `-${fl.value}`}
                    </div>
                ))}
            </div>

            <Modal />
        </div>
    );
}
