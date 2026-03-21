//==================================
// src/components/MapWithCamera.tsx
//==================================

"use client";

import React from "react";
import { useSelector, shallowEqual } from "react-redux";
import type { RootState } from "@/store";
import Map from "./Map";
import type { Cell, PlayerState } from "../types";
import { useCombatFloaters } from "../hooks/useCombatFloaters";
import { normalizeAvatarPath } from "../utils/normalizeAvatarPath";
import styles from "../styles/Map.module.css";

interface MapWithCameraProps {
    tileSize: number;
    viewportWidth: number;
    viewportHeight: number;
    myPlayer: PlayerState;
    onCellClick?: (cell: Cell) => void;
    onPlayerClick?: (player: PlayerState) => void;
}

const STEP_ANIM_MS = 260;
const SPRITE_MAX_WIDTH_FACTOR = 1.05;
const SPRITE_MAX_HEIGHT_FACTOR = 1.18;
const PLAYER_IMAGE_OFFSET_X = 2;
const PLAYER_IMAGE_OFFSET_Y = 2;

type SpriteImageMeta = {
    width: number;
    height: number;
};

type SpriteLayout = {
    frameWidth: number;
    frameHeight: number;
    width: number;
    height: number;
    stripWidth: number;
};

type CharacterSpriteConfig = {
    imageKey: string;
    rightWalkSpriteSrc?: string;
    rightWalkFrames?: number;
    leftWalkSpriteSrc?: string;
    leftWalkFrames?: number;
    idleSideRightSpriteSrc?: string;
    idleSideLeftSpriteSrc?: string;
    downWalkSpriteSrc?: string;
    downWalkFrames?: number;
    idleFrontSpriteSrc?: string;
    upWalkSpriteSrc?: string;
    upWalkFrames?: number;
    idleBackSpriteSrc?: string;
};

type MoveDirection = "left" | "right" | "up" | "down";

type ResolveActiveSpritePoseInput = {
    isMoving: boolean;
    activeDir: MoveDirection;
    facingDir: MoveDirection;
    spriteCfg: CharacterSpriteConfig | null;
    horizontalLayout: SpriteLayout | null;
    leftWalkLayout: SpriteLayout | null;
    downLayout: SpriteLayout | null;
    upLayout: SpriteLayout | null;
    idleFrontLayout: SpriteLayout | null;
    idleBackLayout: SpriteLayout | null;
    idleSideRightLayout: SpriteLayout | null;
    idleSideLeftLayout: SpriteLayout | null;
};

type StepAnim = {
    dir: MoveDirection;
    startMs: number;
    durationMs: number;
    from: { x: number; y: number };
    to: { x: number; y: number };
};

type PreparedPlayerRenderData = {
    player: PlayerState;
    renderLeft: number;
    renderTop: number;
    renderWidth: number;
    renderHeight: number;
    activeSpriteSrc?: string;
    activeLayout: SpriteLayout | null;
    shouldMirror: boolean;
    isActiveUser: boolean;
};

type PlayerLayerProps = {
    players: PlayerState[];
    playerPosition: { x: number; y: number };
    sightRange: number;
    tileSize: number;
    gap: number;
    active_user: number | null | undefined;
    onPlayerClick?: (player: PlayerState) => void;
    spriteMetaBySrc: Record<string, SpriteImageMeta>;
};

const CHARACTER_SPRITES: CharacterSpriteConfig[] = [
    {
        imageKey: "guardian",
        rightWalkSpriteSrc: "/guardian/walk-right-1.png",
        rightWalkFrames: 1,
        leftWalkSpriteSrc: "/guardian/walk-left-1.png",
        leftWalkFrames: 1,
        idleSideRightSpriteSrc: "/guardian/idle-side-right-1.png",
        idleSideLeftSpriteSrc: "/guardian/idle-side-left-1.png",
        downWalkSpriteSrc: "/guardian/walk-down-1.png",
        downWalkFrames: 1,
        idleFrontSpriteSrc: "/guardian/idle-front-1.png",
        upWalkSpriteSrc: "/guardian/walk-up-1.png",
        upWalkFrames: 1,
        idleBackSpriteSrc: "/guardian/idle-back-1.png",
    },
    {
        imageKey: "ranger",
        rightWalkSpriteSrc: "/ranger/walk-right-1.png",
        rightWalkFrames: 1,
        leftWalkSpriteSrc: "/ranger/walk-left-1.png",
        leftWalkFrames: 1,
        idleSideRightSpriteSrc: "/ranger/idle-side-right-1.png",
        idleSideLeftSpriteSrc: "/ranger/idle-side-left-1.png",
        downWalkSpriteSrc: "/ranger/walk-down-1.png",
        downWalkFrames: 1,
        idleFrontSpriteSrc: "/ranger/idle-front-1.png",
        upWalkSpriteSrc: "/ranger/walk-up-1.png",
        upWalkFrames: 1,
        idleBackSpriteSrc: "/ranger/idle-back-1.png",
    },
    {
        imageKey: "berserk",
        rightWalkSpriteSrc: "/berserk/walk-right-2.png",
        rightWalkFrames: 1,
        leftWalkSpriteSrc: "/berserk/walk-left-2.png",
        leftWalkFrames: 1,
        idleSideRightSpriteSrc: "/berserk/idle-side-right-2.png",
        idleSideLeftSpriteSrc: "/berserk/idle-side-left-2.png",
        downWalkSpriteSrc: "/berserk/walk-down-2.png",
        downWalkFrames: 1,
        idleFrontSpriteSrc: "/berserk/idle-front-2.png",
        upWalkSpriteSrc: "/berserk/walk-up-2.png",
        upWalkFrames: 1,
        idleBackSpriteSrc: "/berserk/idle-back-2.png",
    },
    {
        imageKey: "mag.webp",
        rightWalkSpriteSrc: "/mag/mag-walk-right-2.png",
        rightWalkFrames: 1,
        leftWalkSpriteSrc: "/mag/mag-walk-left-2.png",
        leftWalkFrames: 1,
        idleSideRightSpriteSrc: "/mag/mag-idle-side-right-2.png",
        idleSideLeftSpriteSrc: "/mag/mag-idle-side-left-2.png",
        downWalkSpriteSrc: "/mag/mag-walk-down-2.png",
        downWalkFrames: 1,
        idleFrontSpriteSrc: "/mag/mag-idle-front-2.png",
        upWalkSpriteSrc: "/mag/mag-walk-up-2.png",
        upWalkFrames: 1,
        idleBackSpriteSrc: "/mag/mag-idle-back-2.png",
    },
];

function getSpriteConfig(playerImage?: string): CharacterSpriteConfig | null {
    const normalizedPlayerImage = normalizeAvatarPath(playerImage);
    if (!normalizedPlayerImage) return null;

    return (
        CHARACTER_SPRITES.find((cfg) =>
            normalizedPlayerImage.includes(cfg.imageKey),
        ) ?? null
    );
}

function easeInOutCubic(progress: number) {
    return progress < 0.5
        ? 4 * progress * progress * progress
        : 1 - Math.pow(-2 * progress + 2, 3) / 2;
}

function resolveActiveSpritePose({
    isMoving,
    activeDir,
    facingDir,
    spriteCfg,
    horizontalLayout,
    leftWalkLayout,
    downLayout,
    upLayout,
    idleFrontLayout,
    idleBackLayout,
    idleSideRightLayout,
    idleSideLeftLayout,
}: ResolveActiveSpritePoseInput) {
    let activeSpriteSrc: string | undefined;
    let activeLayout: SpriteLayout | null = null;
    let shouldMirror = false;

    if (isMoving) {
        if (activeDir === "left" || activeDir === "right") {
            if (activeDir === "left" && spriteCfg?.leftWalkSpriteSrc) {
                activeSpriteSrc = spriteCfg.leftWalkSpriteSrc;
                activeLayout = leftWalkLayout;
                shouldMirror = false;
            } else {
                activeSpriteSrc = spriteCfg?.rightWalkSpriteSrc;
                activeLayout = horizontalLayout;
                shouldMirror = activeDir === "left";
            }
        } else if (activeDir === "down") {
            activeSpriteSrc = spriteCfg?.downWalkSpriteSrc;
            activeLayout = downLayout;
        } else if (activeDir === "up") {
            activeSpriteSrc = spriteCfg?.upWalkSpriteSrc;
            activeLayout = upLayout;
        }
    } else if (facingDir === "right") {
        activeSpriteSrc = spriteCfg?.idleSideRightSpriteSrc;
        activeLayout = idleSideRightLayout;
    } else if (facingDir === "left") {
        activeSpriteSrc = spriteCfg?.idleSideLeftSpriteSrc;
        activeLayout = idleSideLeftLayout;
    } else if (facingDir === "up") {
        activeSpriteSrc = spriteCfg?.idleBackSpriteSrc;
        activeLayout = idleBackLayout;
    } else {
        activeSpriteSrc = spriteCfg?.idleFrontSpriteSrc;
        activeLayout = idleFrontLayout;
    }

    return { activeSpriteSrc, activeLayout, shouldMirror };
}

const PlayerLayer = React.memo(function PlayerLayer({
    players,
    playerPosition,
    sightRange,
    tileSize,
    gap,
    active_user,
    onPlayerClick,
    spriteMetaBySrc,
}: PlayerLayerProps) {
    const previousPositionsRef = React.useRef<
        globalThis.Map<number, { x: number; y: number }>
    >(new globalThis.Map());

    const [stepAnimByPlayer, setStepAnimByPlayer] = React.useState<
        globalThis.Map<number, StepAnim>
    >(new globalThis.Map());

    const [facingDirByPlayer, setFacingDirByPlayer] = React.useState<
        globalThis.Map<number, MoveDirection>
    >(new globalThis.Map());

    const [animTick, setAnimTick] = React.useState(0);

    React.useEffect(() => {
        if (stepAnimByPlayer.size === 0) return;

        let frameId = 0;

        const tick = () => {
            setAnimTick((v) => v + 1);
            frameId = window.requestAnimationFrame(tick);
        };

        frameId = window.requestAnimationFrame(tick);

        return () => window.cancelAnimationFrame(frameId);
    }, [stepAnimByPlayer]);

    React.useEffect(() => {
        const prev = previousPositionsRef.current;
        const movedNow: Array<{
            id: number;
            dir: MoveDirection;
            from: { x: number; y: number };
            to: { x: number; y: number };
        }> = [];

        for (const p of players) {
            const last = prev.get(p.user_id);

            if (last && (last.x !== p.position.x || last.y !== p.position.y)) {
                let dir: MoveDirection = "down";

                if (p.position.x < last.x) dir = "left";
                else if (p.position.x > last.x) dir = "right";
                else if (p.position.y < last.y) dir = "up";
                else if (p.position.y > last.y) dir = "down";

                movedNow.push({
                    id: p.user_id,
                    dir,
                    from: { x: last.x, y: last.y },
                    to: { x: p.position.x, y: p.position.y },
                });
            }

            prev.set(p.user_id, { x: p.position.x, y: p.position.y });
        }

        if (movedNow.length === 0) return;

        const now = Date.now();

        setStepAnimByPlayer((old) => {
            const next = new globalThis.Map(old);

            movedNow.forEach(({ id, dir, from, to }) => {
                next.set(id, {
                    dir,
                    startMs: now,
                    durationMs: STEP_ANIM_MS,
                    from,
                    to,
                });
            });

            return next;
        });

        setFacingDirByPlayer((old) => {
            const next = new globalThis.Map(old);
            movedNow.forEach(({ id, dir }) => next.set(id, dir));
            return next;
        });

        const timer = window.setTimeout(() => {
            setStepAnimByPlayer((old) => {
                const next = new globalThis.Map(old);
                movedNow.forEach(({ id }) => next.delete(id));
                return next;
            });
        }, STEP_ANIM_MS + 30);

        return () => window.clearTimeout(timer);
    }, [players]);

    const getStepProgress = React.useCallback(
        (step?: StepAnim, nowMs?: number) => {
            if (!step) return 1;

            const elapsed = Math.max(0, (nowMs ?? Date.now()) - step.startMs);
            return easeInOutCubic(Math.min(1, elapsed / step.durationMs));
        },
        [],
    );

    const getSpriteLayout = React.useCallback(
        (src: string | undefined, frames: number): SpriteLayout | null => {
            if (!src) return null;

            const meta = spriteMetaBySrc[src];
            if (!meta) return null;

            const safeFrames = Math.max(1, frames);
            const frameWidth = meta.width / safeFrames;
            const frameHeight = meta.height;
            const maxRenderWidth = tileSize * SPRITE_MAX_WIDTH_FACTOR;
            const maxRenderHeight = tileSize * SPRITE_MAX_HEIGHT_FACTOR;

            const scaleByHeight = maxRenderHeight / Math.max(1, frameHeight);
            const widthAtHeightScale = frameWidth * scaleByHeight;
            const scale =
                widthAtHeightScale > maxRenderWidth
                    ? maxRenderWidth / Math.max(1, frameWidth)
                    : scaleByHeight;

            return {
                frameWidth,
                frameHeight,
                width: frameWidth * scale,
                height: frameHeight * scale,
                stripWidth: meta.width * scale,
            };
        },
        [spriteMetaBySrc, tileSize],
    );

    const preparedPlayers = React.useMemo<PreparedPlayerRenderData[]>(() => {
        const now = Date.now();
        const layoutCache = new globalThis.Map<string, SpriteLayout | null>();
        const result: PreparedPlayerRenderData[] = [];

        const getCachedLayout = (src: string | undefined, frames: number) => {
            const key = `${src ?? "none"}::${frames}`;

            if (layoutCache.has(key)) {
                return layoutCache.get(key) ?? null;
            }

            const layout = getSpriteLayout(src, frames);
            layoutCache.set(key, layout);
            return layout;
        };

        for (const player of players) {
            const dx = Math.abs(player.position.x - playerPosition.x);
            const dy = Math.abs(player.position.y - playerPosition.y);
            const playerVisible = dx <= sightRange && dy <= sightRange;

            if (!playerVisible) continue;

            const spriteCfg = getSpriteConfig(player.image);
            const stepAnim = stepAnimByPlayer.get(player.user_id);
            const isMoving =
                !!stepAnim && now - stepAnim.startMs < stepAnim.durationMs;
            const facingDir = facingDirByPlayer.get(player.user_id) ?? "down";

            const activeDir: MoveDirection =
                isMoving && stepAnim ? stepAnim.dir : facingDir;

            const stepProgress = getStepProgress(stepAnim, now);

            const renderPos = stepAnim
                ? {
                      x:
                          stepAnim.from.x +
                          (stepAnim.to.x - stepAnim.from.x) * stepProgress,
                      y:
                          stepAnim.from.y +
                          (stepAnim.to.y - stepAnim.from.y) * stepProgress,
                  }
                : player.position;

            const horizontalLayout = getCachedLayout(
                spriteCfg?.rightWalkSpriteSrc,
                spriteCfg?.rightWalkFrames ?? 1,
            );
            const leftWalkLayout = getCachedLayout(
                spriteCfg?.leftWalkSpriteSrc,
                spriteCfg?.leftWalkFrames ?? 1,
            );
            const downLayout = getCachedLayout(
                spriteCfg?.downWalkSpriteSrc,
                spriteCfg?.downWalkFrames ?? 1,
            );
            const upLayout = getCachedLayout(
                spriteCfg?.upWalkSpriteSrc,
                spriteCfg?.upWalkFrames ?? 1,
            );
            const idleFrontLayout = getCachedLayout(
                spriteCfg?.idleFrontSpriteSrc,
                1,
            );
            const idleBackLayout = getCachedLayout(
                spriteCfg?.idleBackSpriteSrc,
                1,
            );
            const idleSideRightLayout = getCachedLayout(
                spriteCfg?.idleSideRightSpriteSrc,
                1,
            );
            const idleSideLeftLayout = getCachedLayout(
                spriteCfg?.idleSideLeftSpriteSrc,
                1,
            );

            const { activeSpriteSrc, activeLayout, shouldMirror } =
                resolveActiveSpritePose({
                    isMoving,
                    activeDir,
                    facingDir,
                    spriteCfg,
                    horizontalLayout,
                    leftWalkLayout,
                    downLayout,
                    upLayout,
                    idleFrontLayout,
                    idleBackLayout,
                    idleSideRightLayout,
                    idleSideLeftLayout,
                });

            const renderWidth = activeLayout?.width ?? tileSize;
            const renderHeight = activeLayout?.height ?? tileSize;
            const renderLeft =
                renderPos.x * (tileSize + gap) +
                tileSize / 2 -
                renderWidth / 2 +
                PLAYER_IMAGE_OFFSET_X;
            const renderTop =
                renderPos.y * (tileSize + gap) +
                tileSize -
                renderHeight +
                PLAYER_IMAGE_OFFSET_Y;

            result.push({
                player,
                renderLeft,
                renderTop,
                renderWidth,
                renderHeight,
                activeSpriteSrc,
                activeLayout,
                shouldMirror,
                isActiveUser: player.user_id === active_user,
            });
        }

        return result;
    }, [
        players,
        playerPosition.x,
        playerPosition.y,
        sightRange,
        stepAnimByPlayer,
        facingDirByPlayer,
        active_user,
        tileSize,
        gap,
        getStepProgress,
        getSpriteLayout,
        animTick,
    ]);

    return (
        <>
            {preparedPlayers.map(
                ({
                    player,
                    renderLeft,
                    renderTop,
                    renderWidth,
                    renderHeight,
                    activeSpriteSrc,
                    activeLayout,
                    shouldMirror,
                    isActiveUser,
                }) => (
                    <div
                        key={player.user_id}
                        title={player.name}
                        onClick={() => onPlayerClick?.(player)}
                        style={{
                            position: "absolute",
                            left: renderLeft,
                            top: renderTop,
                            width: renderWidth,
                            height: renderHeight,
                            border: isActiveUser ? "2px solid gold" : "none",
                            boxSizing: "border-box",
                            zIndex: 10,
                            overflow: "hidden",
                            borderRadius: 4,
                        }}
                    >
                        {activeLayout && activeSpriteSrc ? (
                            <div
                                aria-label={player.name}
                                style={{
                                    width: activeLayout.width,
                                    height: activeLayout.height,
                                    backgroundImage: `url(${activeSpriteSrc})`,
                                    backgroundRepeat: "no-repeat",
                                    backgroundSize: `${activeLayout.stripWidth}px ${activeLayout.height}px`,
                                    backgroundPositionX: "0px",
                                    transform: shouldMirror
                                        ? "scaleX(-1)"
                                        : "none",
                                    transformOrigin: "center",
                                    imageRendering: "pixelated",
                                }}
                            />
                        ) : (
                            <img
                                src={normalizeAvatarPath(player.image)}
                                alt={player.name}
                                draggable={false}
                                style={{
                                    width: "100%",
                                    height: "100%",
                                    objectFit: "contain",
                                    pointerEvents: "none",
                                }}
                            />
                        )}
                    </div>
                ),
            )}
        </>
    );
});

export default function MapWithCamera({
    tileSize: inputTileSize,
    viewportWidth,
    viewportHeight,
    myPlayer,
    onCellClick,
    onPlayerClick,
}: MapWithCameraProps) {
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
    const sightRange = myPlayer?.sightRange ?? 3;
    const tileSize = Number(inputTileSize) || 60;
    const safeMapWidth = Number(mapWidth) || 15;
    const safeMapHeight = Number(mapHeight) || 15;
    const gap = 1;

    const { offsetX, offsetY } = React.useMemo(() => {
        let nextOffsetX =
            viewportWidth / 2 -
            (playerPosition.x * (tileSize + gap) + tileSize / 2);
        let nextOffsetY =
            viewportHeight / 2 -
            (playerPosition.y * (tileSize + gap) + tileSize / 2);

        const totalWidth = safeMapWidth * tileSize + (safeMapWidth - 1) * gap;
        const totalHeight = safeMapHeight * tileSize + (safeMapHeight - 1) * gap;

        nextOffsetX = Math.min(
            0,
            Math.max(viewportWidth - totalWidth, nextOffsetX),
        );
        nextOffsetY = Math.min(
            0,
            Math.max(viewportHeight - totalHeight, nextOffsetY),
        );

        return {
            offsetX: nextOffsetX,
            offsetY: nextOffsetY,
        };
    }, [
        viewportWidth,
        viewportHeight,
        playerPosition.x,
        playerPosition.y,
        tileSize,
        gap,
        safeMapWidth,
        safeMapHeight,
    ]);

    const { floaters, flashes } = useCombatFloaters(players, grid);

    const spriteSources = React.useMemo(
        () =>
            Array.from(
                new Set(
                    CHARACTER_SPRITES.flatMap((cfg) => [
                        cfg.rightWalkSpriteSrc,
                        cfg.leftWalkSpriteSrc,
                        cfg.idleSideRightSpriteSrc,
                        cfg.idleSideLeftSpriteSrc,
                        cfg.downWalkSpriteSrc,
                        cfg.idleFrontSpriteSrc,
                        cfg.upWalkSpriteSrc,
                        cfg.idleBackSpriteSrc,
                    ]).filter((src): src is string => !!src),
                ),
            ),
        [],
    );

    const [spriteMetaBySrc, setSpriteMetaBySrc] = React.useState<
        Record<string, SpriteImageMeta>
    >({});

    React.useEffect(() => {
        let cancelled = false;

        spriteSources.forEach((src) => {
            if (spriteMetaBySrc[src]) return;

            const image = new Image();

            image.onload = () => {
                if (cancelled) return;

                setSpriteMetaBySrc((old) => {
                    if (old[src]) return old;

                    return {
                        ...old,
                        [src]: {
                            width: image.naturalWidth,
                            height: image.naturalHeight,
                        },
                    };
                });
            };

            image.src = src;
        });

        return () => {
            cancelled = true;
        };
    }, [spriteMetaBySrc, spriteSources]);

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
                    sightRange={sightRange}
                    playerPosition={playerPosition}
                    onCellClick={onCellClick}
                    players={players}
                />

                <PlayerLayer
                    players={players}
                    playerPosition={playerPosition}
                    sightRange={sightRange}
                    tileSize={tileSize}
                    gap={gap}
                    active_user={active_user}
                    onPlayerClick={onPlayerClick}
                    spriteMetaBySrc={spriteMetaBySrc}
                />

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
        </div>
    );
}
