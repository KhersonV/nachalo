//==================================
// src/components/GameController.tsx
//==================================

import { API_BASE } from "@/utils/serviceUrls";
import { useSelector, useDispatch } from "react-redux";
import React, { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "../contexts/AuthContext";
import MapWithCamera from "./MapWithCamera";
import MiniMap from "./MiniMap";
import Controls from "./Controls";
import EndTurnButton from "./EndTurnButton";
import TurnIndicator from "./TurnIndicator";
import Inventory from "./Inventory";
import PlayerHUD from "./PlayerHUD";
import { ObjectHUD } from "./ObjectHUD";
import ObjectiveTracker from "./ObjectiveTracker";
import ActionLog from "./ActionLog";
import QuestArtifactAlert from "./QuestArtifactAlert";
import styles from "../styles/GameController.module.css";
import objectHudStyles from "../styles/ObjectHUD.module.css";
import type { RootState } from "../store";
import type { Cell, PlayerState } from "../types";
import {
  setInstanceId,
  setActiveUser,
  setQuestFoundNotification,
  addActionLogEntry,
} from "../store/slices/gameSlice";
import { usePlayerActions } from "../hooks/usePlayerActions";
import { useGameKeyboard } from "../hooks/useGameKeyboard";
import { humanizeActionError } from "@/utils/actionLog";

type PlacementStructureType = "scout_tower" | "turret" | "wall";

type PlacementModeState = {
  blueprintKey: string;
  structureType: PlacementStructureType;
};

const structureTypeByBlueprintKey: Record<string, PlacementStructureType> = {
  blueprint_scout_tower: "scout_tower",
  blueprint_turret: "turret",
  blueprint_wall: "wall",
};

// Default max HP for structures when backend doesn't expose maxHealth
const STRUCTURE_DEFAULT_MAX_HEALTH: Record<PlacementStructureType, number> = {
  scout_tower: 30,
  turret: 30,
  wall: 30,
};

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
}

function formatDuration(seconds: number) {
  const safeSeconds = Math.max(0, Math.ceil(seconds));
  const minutes = Math.floor(safeSeconds / 60);
  const remainder = safeSeconds % 60;

  return `${minutes}:${String(remainder).padStart(2, "0")}`;
}

function isCompactPhoneViewport(width: number, height: number) {
  const isPhonePortrait = width <= 760 && height >= width;
  const isPhoneLandscape = width <= 920 && height < width;
  return isPhonePortrait || isPhoneLandscape;
}

interface GameControllerProps {
  instanceId: string;
}

export default function GameController({ instanceId }: GameControllerProps) {
  const dispatch = useDispatch();
  const router = useRouter();
  const state = useSelector((state: RootState) => state.game);
  const { user } = useAuth();
  

  useEffect(() => {
    if (state.instanceId !== instanceId) {
      dispatch(setInstanceId(instanceId));
    }
  }, [instanceId, dispatch, state.instanceId]);

  const [showInventory, setShowInventory] = useState(false);
  const [placementMode, setPlacementMode] = useState<PlacementModeState | null>(
    null,
  );

  // HUD для объектов (монстры, постройки, другие игроки)
  const [objectHUD, setObjectHUD] = useState<{
    type: "monster" | "structure" | "player" | "object";
    name: string;
    details?: string;
    health?: number;
    maxHealth?: number;
    energy?: number;
    maxEnergy?: number;
    attack?: number;
    defense?: number;
    sightRange?: number;
    structureType?: "scout_tower" | "turret" | "wall";
    userId?: number;
    groupId?: number;
    x?: number;
    y?: number;
  } | null>(null);
  const [showTurnModal, setShowTurnModal] = useState(false);
  const prevIsMyTurnRef = React.useRef(false);
  const turnModalTimerRef = React.useRef<ReturnType<typeof setTimeout> | null>(
    null,
  );
  const [showQuestAlert, setShowQuestAlert] = useState(false);
  const [showQuestFoundAlert, setShowQuestFoundAlert] = useState(false);
  const [canOpenStats, setCanOpenStats] = useState(false);
  const [hasEscaped, setHasEscaped] = useState(false);
  const [isActionLogExpanded, setIsActionLogExpanded] = useState(false);
  const [disconnectedDeadlines, setDisconnectedDeadlines] = useState<
    Record<number, number>
  >({});
  const [showDisconnectPanel, setShowDisconnectPanel] = useState(true);
  const [nowMs, setNowMs] = useState<number>(Date.now());
  const [mapViewport, setMapViewport] = useState({
    width: 800,
    height: 600,
    tileSize: 80,
  });
  const [isCompactViewport, setIsCompactViewport] = useState(() =>
    typeof window !== "undefined"
      ? isCompactPhoneViewport(window.innerWidth, window.innerHeight)
      : false,
  );
  const [showMiniMap, setShowMiniMap] = useState(() =>
    typeof window !== "undefined"
      ? !isCompactPhoneViewport(window.innerWidth, window.innerHeight)
      : true,
  );
  const [minimapFocusPoint, setMinimapFocusPoint] = useState<{
    x: number;
    y: number;
  } | null>(null);
  const [minimapPingPoint, setMinimapPingPoint] = useState<{
    x: number;
    y: number;
  } | null>(null);
  const questAlertShownRef = React.useRef(false);
  const minimapPreferenceLockedRef = React.useRef(false);
  const turnStartMsRef = React.useRef<number>(Date.now());
  const autoEndTurnInFlightRef = React.useRef(false);
  const [profileModalUserId, setProfileModalUserId] = useState<number | null>(
    null,
  );
  const [profileModalData, setProfileModalData] = useState<any | null>(null);
  const [profileModalLoading, setProfileModalLoading] = useState(false);
  const [profileModalError, setProfileModalError] = useState("");
  const [profileModalFriendLoading, setProfileModalFriendLoading] =
    useState(false);
  const [profileModalOutgoingSent, setProfileModalOutgoingSent] =
    useState(false);

  useEffect(() => {
    if (typeof window === "undefined") return;

    const baseViewportWidth = 800;
    const baseViewportHeight = 600;
    const baseTileSize = 80;
    const mapGap = 1;

    const buildViewport = (
      availableWidth: number,
      availableHeight: number,
      columns: number,
      rows: number,
      minTileSize: number,
    ) => {
      const safeWidth = Math.max(220, availableWidth);
      const safeHeight = Math.max(220, availableHeight);
      const widthTile = Math.floor(
        (safeWidth - (columns - 1) * mapGap) / columns,
      );
      const heightTile = Math.floor((safeHeight - (rows - 1) * mapGap) / rows);
      const tileSize = clamp(
        Math.min(widthTile, heightTile, baseTileSize),
        minTileSize,
        baseTileSize,
      );

      return {
        width: columns * tileSize + (columns - 1) * mapGap,
        height: rows * tileSize + (rows - 1) * mapGap,
        tileSize,
      };
    };

    const updateViewport = () => {
      const screenW = window.innerWidth;
      const screenH = window.innerHeight;
      const isPhonePortrait = screenW <= 760 && screenH >= screenW;
      const isPhoneLandscape = screenW <= 920 && screenH < screenW;
      const isMobile = screenW <= 900;
      const nextCompactViewport = isCompactPhoneViewport(screenW, screenH);

      setIsCompactViewport(nextCompactViewport);
      setShowMiniMap((current) =>
        minimapPreferenceLockedRef.current ? current : !nextCompactViewport,
      );

      if (isPhonePortrait) {
        setMapViewport(buildViewport(screenW - 16, screenH - 232, 7, 6, 42));
        return;
      }

      if (isPhoneLandscape) {
        setMapViewport(buildViewport(screenW - 24, screenH - 122, 8, 5, 36));
        return;
      }

      if (!isMobile) {
        setMapViewport({
          width: baseViewportWidth,
          height: baseViewportHeight,
          tileSize: baseTileSize,
        });
        return;
      }

      // Keep the same logical map window for all devices and only scale
      // the rendered result on small screens.

      const availableWidth = screenW;
      const availableHeight = screenH;

      const scale = Math.max(
        0.42,
        Math.min(
          1,
          Math.min(
            availableWidth / baseViewportWidth,
            availableHeight / baseViewportHeight,
          ),
        ),
      );

      setMapViewport({
        width: Math.floor(baseViewportWidth * scale),
        height: Math.floor(baseViewportHeight * scale),
        tileSize: Math.max(28, Math.floor(baseTileSize * scale)),
      });
    };

    updateViewport();
    window.addEventListener("resize", updateViewport);
    window.addEventListener("orientationchange", updateViewport);

    return () => {
      window.removeEventListener("resize", updateViewport);
      window.removeEventListener("orientationchange", updateViewport);
    };
  }, []);

  useEffect(() => {
    // Новый матч: сбрасываем флаг и возможный кэш прошлой статистики.
    setCanOpenStats(false);
    setHasEscaped(false);
    if (typeof window !== "undefined") {
      sessionStorage.removeItem("lastMatchPlayerStats");
      sessionStorage.removeItem("lastMatchEscaped");
      sessionStorage.removeItem("lastMatchEliminated");
    }
  }, [instanceId]);

  useEffect(() => {
    const handleStatsReady = () => setCanOpenStats(true);
    if (typeof window !== "undefined") {
      window.addEventListener("match-stats-ready", handleStatsReady);
      if (sessionStorage.getItem("lastMatchPlayerStats")) {
        setCanOpenStats(true);
      }
    }
    return () => {
      if (typeof window !== "undefined") {
        window.removeEventListener("match-stats-ready", handleStatsReady);
      }
    };
  }, []);

  useEffect(() => {
    const tick = window.setInterval(() => {
      setNowMs(Date.now());
    }, 1000);
    return () => {
      window.clearInterval(tick);
    };
  }, []);

  // Load public profile for modal when requested
  useEffect(() => {
    if (!profileModalUserId) {
      setProfileModalData(null);
      setProfileModalError("");
      setProfileModalLoading(false);
      setProfileModalOutgoingSent(false);
      return;
    }

    let alive = true;
    setProfileModalLoading(true);
    setProfileModalError("");
    setProfileModalData(null);

    (async () => {
      try {
        const res = await fetch(
          `${API_BASE}/game/profile/${profileModalUserId}`,
          {
            headers: {
              Authorization: `Bearer ${user?.token}`,
            },
            cache: "no-store",
          },
        );
        if (!res.ok) {
          const t = await res.text();
          throw new Error(t || "Failed to load profile");
        }
        const data = await res.json();
        if (!alive) return;
        setProfileModalData(data.data ?? data);
      } catch (e: any) {
        if (!alive) return;
        setProfileModalError(e?.message || "Failed to load profile");
      } finally {
        if (!alive) return;
        setProfileModalLoading(false);
      }
    })();

    return () => {
      alive = false;
    };
  }, [profileModalUserId, user?.token]);

  // Reset local turn countdown whenever turn ownership changes.
  useEffect(() => {
    turnStartMsRef.current = Date.now();
    autoEndTurnInFlightRef.current = false;
  }, [state.active_user, state.turnNumber]);

  // Persist turn start time across navigation/refresh so client-side timer
  // doesn't reset when GameController unmounts/remounts.
  useEffect(() => {
    const key = `turnStartMs:${instanceId}`;
    if (typeof window !== "undefined") {
      try {
        const v = sessionStorage.getItem(key);
        if (v) turnStartMsRef.current = Number(v);
      } catch (e) {
        // ignore
      }
    }
    return () => {
      if (typeof window !== "undefined") {
        try {
          sessionStorage.setItem(key, String(turnStartMsRef.current));
        } catch (e) {
          // ignore
        }
      }
    };
  }, [instanceId]);

  useEffect(() => {
    const onDisconnected = (event: Event) => {
      const custom = event as CustomEvent<{
        userId?: number;
        graceMs?: number;
      }>;
      const userId = custom.detail?.userId;
      const graceMs = custom.detail?.graceMs ?? 180000;
      if (!userId || userId === user?.id) return;
      setDisconnectedDeadlines((prev) => ({
        ...prev,
        [userId]: Date.now() + graceMs,
      }));
    };

    const onReconnected = (event: Event) => {
      const custom = event as CustomEvent<{ userId?: number }>;
      const userId = custom.detail?.userId;
      if (!userId) return;
      setDisconnectedDeadlines((prev) => {
        const next = { ...prev };
        delete next[userId];
        return next;
      });
    };

    window.addEventListener("player-disconnected", onDisconnected);
    window.addEventListener("player-reconnected", onReconnected);

    return () => {
      window.removeEventListener("player-disconnected", onDisconnected);
      window.removeEventListener("player-reconnected", onReconnected);
    };
  }, [user?.id]);

  useEffect(() => {
    const onMyDefeat = () => {
      dispatch(
        setQuestFoundNotification({
          eventType: "MY_PLAYER_DEFEATED",
          message:
            'Your character has died. Click "To Stats" to open the results screen.',
          instanceId,
          userId: user?.id,
        }),
      );
    };

    window.addEventListener("my-player-defeated", onMyDefeat);
    return () => {
      window.removeEventListener("my-player-defeated", onMyDefeat);
    };
  }, [dispatch, instanceId, user?.id]);

  // Show the quest artifact alert once when the map loads
  useEffect(() => {
    if (
      !questAlertShownRef.current &&
      state.isMapLoaded &&
      state.questArtifactId !== 0
    ) {
      questAlertShownRef.current = true;
      setShowQuestAlert(true);
    }
  }, [state.isMapLoaded, state.questArtifactId]);

  // Show the same modal when someone finds the quest artifact
  useEffect(() => {
    if (!state.questFoundNotification) return;
    setShowQuestFoundAlert(true);
  }, [state.questFoundNotification]);

  const handleBlueprintPlacementStart = useCallback((blueprintKey: string) => {
    const structureType = structureTypeByBlueprintKey[blueprintKey];
    if (!structureType) return;
    setPlacementMode({ blueprintKey, structureType });
    setShowInventory(false);
  }, []);

  const pushActionLog = useCallback(
    (
      message: string,
      category: "error" | "movement" | "attack" | "portal" | "turn" = "error",
      tone: "info" | "success" | "warning" | "danger" = "warning",
    ) => {
      dispatch(
        addActionLogEntry({
          category,
          tone,
          message,
          dedupeKey: `local:${category}:${message}:${Math.floor(Date.now() / 1400)}`,
        }),
      );
    },
    [dispatch],
  );

  const {
    myPlayer,
    isMyTurn,
    handleMoveOrAttack,
    handleCellClick,
    handlePlayerClick,
    openBarrel,
    collectResource,
    fightMonster,
    fightPlayer,
  } = usePlayerActions(instanceId, user, state, {
    blueprintKey: placementMode?.blueprintKey ?? null,
    structureType: placementMode?.structureType ?? null,
    onPlaced: () => setPlacementMode(null),
    onError: (message: string) => {
      pushActionLog(humanizeActionError(message), "error", "warning");
    },
  });

  const shouldRenderCompactMiniMap = isCompactViewport && !!myPlayer;
  const isCompactActionLogVisible =
    isCompactViewport &&
    !showMiniMap &&
    !objectHUD &&
    state.actionLog.length > 0;
  const shouldShowActionLog =
    !isCompactViewport || (!showMiniMap && !objectHUD);
  const shouldShowObjectHUD =
    !!objectHUD && (!isCompactViewport || !showMiniMap);

  const minimapViewportCells = React.useMemo(() => {
    const step = mapViewport.tileSize + 1;
    return {
      width: Math.max(1, Math.ceil(mapViewport.width / step)),
      height: Math.max(1, Math.ceil(mapViewport.height / step)),
    };
  }, [mapViewport.height, mapViewport.tileSize, mapViewport.width]);

  const handleMiniMapPing = useCallback((point: { x: number; y: number }) => {
    setMinimapFocusPoint(point);
    setMinimapPingPoint(point);
  }, []);

  const handleMiniMapVisibilityToggle = useCallback(() => {
    minimapPreferenceLockedRef.current = true;
    setShowMiniMap((current) => {
      const next = !current;
      if (next && isCompactViewport) {
        setShowInventory(false);
        setObjectHUD(null);
        setIsActionLogExpanded(false);
      }
      return next;
    });
  }, [isCompactViewport]);

  const handleInventoryToggle = useCallback(() => {
    setShowInventory((current) => {
      const next = !current;
      if (next && isCompactViewport) {
        setShowMiniMap(false);
        setObjectHUD(null);
        setIsActionLogExpanded(false);
      }
      return next;
    });
  }, [isCompactViewport]);

  const presentObjectHUD = useCallback(
    (
      nextHud: {
        type: "monster" | "structure" | "player" | "object";
        name: string;
        details?: string;
        health?: number;
        maxHealth?: number;
        energy?: number;
        maxEnergy?: number;
        attack?: number;
        defense?: number;
        sightRange?: number;
        structureType?: "scout_tower" | "turret" | "wall";
        userId?: number;
        groupId?: number;
        x?: number;
        y?: number;
      } | null,
    ) => {
      if (nextHud && isCompactViewport) {
        setShowMiniMap(false);
        setShowInventory(false);
        setIsActionLogExpanded(false);
      }
      setObjectHUD(nextHud);
    },
    [isCompactViewport],
  );

  useEffect(() => {
    if (!minimapFocusPoint) return;

    const timeoutId = window.setTimeout(() => {
      setMinimapFocusPoint(null);
    }, 2200);

    return () => {
      window.clearTimeout(timeoutId);
    };
  }, [minimapFocusPoint]);

  useEffect(() => {
    if (!minimapPingPoint) return;

    const timeoutId = window.setTimeout(() => {
      setMinimapPingPoint(null);
    }, 1600);

    return () => {
      window.clearTimeout(timeoutId);
    };
  }, [minimapPingPoint]);

  useEffect(() => {
    setMinimapFocusPoint(null);
    setMinimapPingPoint(null);
    setIsActionLogExpanded(false);
  }, [instanceId, myPlayer?.user_id]);

  const handleActionLogExpandedChange = useCallback(
    (expanded: boolean) => {
      setIsActionLogExpanded(expanded);
      if (expanded && isCompactViewport) {
        setShowMiniMap(false);
        setObjectHUD(null);
        setShowInventory(false);
      }
    },
    [isCompactViewport],
  );

  const handleMapPlayerClick = useCallback(
    async (targetPlayer: PlayerState) => {
      if (!myPlayer) return;
      // If the clicked player is ourselves, open the object HUD showing
      // our current stats so the user can view profile/HUD by click.
      if (targetPlayer.user_id === myPlayer.user_id) {
        presentObjectHUD({
          type: "player",
          name: myPlayer.name,
          health: myPlayer.health,
          maxHealth: myPlayer.maxHealth,
          energy: myPlayer.energy,
          maxEnergy: myPlayer.maxEnergy,
          attack: myPlayer.attack,
          defense: myPlayer.defense,
          userId: myPlayer.user_id,
          groupId: myPlayer.group_id,
        });
        return;
      }
      const attackRange = myPlayer.attackRange ?? 1;
      const dist =
        Math.abs(myPlayer.position.x - targetPlayer.position.x) +
        Math.abs(myPlayer.position.y - targetPlayer.position.y);
      if (isMyTurn && dist <= attackRange) {
        await fightPlayer(targetPlayer.user_id);
        return;
      }
      if (isMyTurn && dist > attackRange) {
        pushActionLog("Target is out of range. Move closer to attack.", "attack");
      }
      presentObjectHUD({
        type: "player",
        name: targetPlayer.name,
        health: targetPlayer.health,
        maxHealth: targetPlayer.maxHealth,
        energy: targetPlayer.energy,
        maxEnergy: targetPlayer.maxEnergy,
        attack: targetPlayer.attack,
        defense: targetPlayer.defense,
        userId: targetPlayer.user_id,
        groupId: targetPlayer.group_id,
      });
    },
    [myPlayer, isMyTurn, fightPlayer, presentObjectHUD, pushActionLog],
  );

  const handleMapCellClick = useCallback(
    async (cell: Cell) => {
      if (!myPlayer) return;

      const distance =
        Math.abs(myPlayer.position.x - cell.x) +
        Math.abs(myPlayer.position.y - cell.y);
      const attackRange = myPlayer.attackRange ?? 1;

      if (cell.monster) {
        if (isMyTurn && distance <= attackRange) {
          await fightMonster(cell.x, cell.y);
          return;
        }
        if (isMyTurn && distance > attackRange) {
          pushActionLog("Monster is out of range. Move closer to attack.", "attack");
        }
        presentObjectHUD({
          type: "monster",
          x: cell.x,
          y: cell.y,
          name: cell.monster.name,
          health: cell.monster.health,
          maxHealth: cell.monster.maxHealth ?? cell.monster.health,
          attack: cell.monster.attack,
          defense: cell.monster.defense,
        });
        return;
      }

      if (cell.structure_type) {
        const isOwnStructure =
          cell.structure_owner_user_id === myPlayer.user_id;
        if (
          isMyTurn &&
          distance === 1 &&
          !isOwnStructure &&
          !cell.is_under_construction
        ) {
          await handleCellClick(cell);
          return;
        }
        presentObjectHUD({
          type: "structure",
          x: cell.x,
          y: cell.y,
          name:
            cell.structure_type === "scout_tower"
              ? "Scout Tower"
              : cell.structure_type === "turret"
                ? "Turret"
                : "Wall",
          health: cell.structure_health,
          maxHealth:
            STRUCTURE_DEFAULT_MAX_HEALTH[
              cell.structure_type as PlacementStructureType
            ] ?? cell.structure_health,
          defense: cell.structure_defense,
          attack: cell.structure_attack,
          structureType: cell.structure_type as PlacementStructureType,
          sightRange: cell.structure_type === "scout_tower" ? 1 : undefined,
        });
        return;
      }

      if (cell.resource) {
        if (isMyTurn && distance === 1) {
          await handleCellClick(cell);
          return;
        }
        if (isMyTurn && distance !== 1) {
          pushActionLog("Move next to the resource to collect it.", "movement");
        }
        presentObjectHUD({
          type: "object",
          name: `Resource: ${cell.resource.type}`,
          details: cell.resource.description || "Useful resource",
        });
        return;
      }

      if (cell.barbel) {
        if (isMyTurn && distance === 1) {
          await handleCellClick(cell);
          return;
        }
        if (isMyTurn && distance !== 1) {
          pushActionLog("Move next to the barrel to open it.", "movement");
        }
        presentObjectHUD({
          type: "object",
          name: "Barrel",
          details: "Can be opened to receive a reward",
        });
        return;
      }

      if (cell.isPortal) {
        if (isMyTurn && distance === 1) {
          await handleCellClick(cell);
          return;
        }
        if (isMyTurn && distance !== 1) {
          pushActionLog("Step onto the portal, then use the action button to escape.", "portal");
        }
        presentObjectHUD({
          type: "object",
          name: "Portal",
          details: "Exit point from the match after conditions are met",
        });
        return;
      }

      if (isMyTurn && distance === 1) {
        await handleCellClick(cell);
        return;
      }

      if (isMyTurn && distance > 1) {
        pushActionLog("Move one adjacent tile at a time.", "movement");
      }
    },
    [
      myPlayer,
      isMyTurn,
      fightMonster,
      handleCellClick,
      presentObjectHUD,
      pushActionLog,
    ],
  );

  // Show centered "YOUR TURN" modal when turn transitions to the current player
  useEffect(() => {
    if (isMyTurn && !prevIsMyTurnRef.current) {
      setShowTurnModal(true);
      if (turnModalTimerRef.current) clearTimeout(turnModalTimerRef.current);
      turnModalTimerRef.current = setTimeout(
        () => setShowTurnModal(false),
        2000,
      );
    }
    prevIsMyTurnRef.current = isMyTurn;
    return () => {
      if (turnModalTimerRef.current) clearTimeout(turnModalTimerRef.current);
    };
  }, [isMyTurn]);

  // Portal exit: call /game/finishMatch with auth token
  const handlePortalExit = useCallback(async () => {
    const token = user?.token;
    if (!token) return;
    setCanOpenStats(false);
    const res = await fetch(`${API_BASE}/game/finishMatch`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({ instanceId }),
    });
    if (!res.ok) {
      const data = await res.json().catch(() => ({}));
      if (data?.error === "quest_artifact_missing") {
        pushActionLog(
          "Find the quest artifact before entering the portal.",
          "portal",
          "warning",
        );
      } else {
        pushActionLog(
          humanizeActionError(data?.error ?? "Could not enter the portal."),
          "portal",
          "warning",
        );
        console.error("[Portal] finishMatch error", res.status, data);
      }
      return;
    }

    const data = await res.json().catch(() => null);
    if (typeof window !== "undefined" && data?.stats) {
      sessionStorage.setItem(
        "lastMatchPlayerStats",
        JSON.stringify(data.stats),
      );
      sessionStorage.setItem("lastMatchEscaped", "1");
      sessionStorage.removeItem("lastMatchEliminated");
      setCanOpenStats(true);
    }
    setHasEscaped(true);
  }, [instanceId, pushActionLog, user]);

  // Можно оптимизировать: вынести в useCallback
  const handleAction = useCallback(() => {
    if (!myPlayer) return;
    if (!isMyTurn) {
      pushActionLog("Wait for your turn before acting.", "turn");
      return;
    }
    const currentCell = state.grid.find(
      (cell: any) =>
        cell.x === myPlayer.position.x && cell.y === myPlayer.position.y,
    );
    if (!currentCell) {
      pushActionLog("No action is available on this tile.");
      return;
    }
    if (currentCell.monster) fightMonster(currentCell.x, currentCell.y);
    else if (currentCell.resource)
      collectResource(currentCell.x, currentCell.y);
    else if (currentCell.barbel) openBarrel(currentCell.x, currentCell.y);
    else if (currentCell.isPortal) handlePortalExit();
    else pushActionLog("No action is available on this tile.");
  }, [
    isMyTurn,
    myPlayer,
    state.grid,
    fightMonster,
    collectResource,
    openBarrel,
    handlePortalExit,
    pushActionLog,
  ]);

  useGameKeyboard({
    onMove: handleMoveOrAttack,
    onAction: handleAction,
    onInventory: handleInventoryToggle,
  });

  const handleTurnEnded = useCallback(
    (data: { active_user: number; turnNumber: number; energy: number }) => {
      turnStartMsRef.current = Date.now();
      autoEndTurnInFlightRef.current = false;
      dispatch(
        setActiveUser({
          instanceId,
          active_user: data.active_user,
          turnNumber: data.turnNumber,
          energy: data.energy,
        }),
      );
    },
    [dispatch, instanceId],
  );

  const notificationEventType = state.questFoundNotification?.eventType;

  const isPortalExitNotification =
    notificationEventType === "PLAYER_LEFT_PORTAL";

  const isDeathNotification = notificationEventType === "MY_PLAYER_DEFEATED";
  const isCurrentPlayerDefeated =
    isDeathNotification ||
    (!!user?.id && state.isMapLoaded && state.players.length > 0 && !myPlayer);
  const isObjectiveMatchFinished = canOpenStats && !hasEscaped;
  const questFoundConfirmLabel =
    isDeathNotification || (isPortalExitNotification && canOpenStats)
      ? "To Stats"
      : "Got it";

  const TURN_SECS = 60;
  const turnSecsLeft = Math.max(
    0,
    TURN_SECS - Math.floor((nowMs - turnStartMsRef.current) / 1000),
  );
  const isTurnWarning = turnSecsLeft <= 30;
  const turnTimerText = `${Math.floor(turnSecsLeft / 60)}:${String(turnSecsLeft % 60).padStart(2, "0")}`;
  const waitingTurnInfo = React.useMemo(() => {
    if (!myPlayer || isMyTurn || state.players.length === 0) {
      return null;
    }

    const activePlayers = state.players.filter(
      (player) => typeof player.health !== "number" || player.health > 0,
    );

    if (activePlayers.length === 0) {
      return null;
    }

    const activeIndex = activePlayers.findIndex(
      (player) => player.user_id === state.active_user,
    );
    const myIndex = activePlayers.findIndex(
      (player) => player.user_id === myPlayer.user_id,
    );

    if (activeIndex === -1 || myIndex === -1) {
      return null;
    }

    const stepsUntilMyTurn =
      (myIndex - activeIndex + activePlayers.length) % activePlayers.length;
    const normalizedSteps =
      stepsUntilMyTurn === 0 ? activePlayers.length : stepsUntilMyTurn;
    const playersAhead = Math.max(0, normalizedSteps - 1);
    const etaSeconds =
      turnSecsLeft + Math.max(0, normalizedSteps - 1) * TURN_SECS;

    return {
      activePlayerName: activePlayers[activeIndex]?.name ?? "another player",
      playersAhead,
      etaText: formatDuration(etaSeconds),
    };
  }, [
    isMyTurn,
    myPlayer,
    state.active_user,
    state.players,
    turnSecsLeft,
    TURN_SECS,
  ]);

  // Fallback: if timer reaches 00:00 on client and it's still my turn,
  // force end-turn request to keep gameplay flowing.
  useEffect(() => {
    if (!isMyTurn || !myPlayer || turnSecsLeft > 0) return;
    if (autoEndTurnInFlightRef.current) return;

    const token = user?.token;
    if (!token) return;

    autoEndTurnInFlightRef.current = true;

    const requestAutoEndTurn = async () => {
      try {
        const response = await fetch(`${API_BASE}/game/endTurn`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({
            user_id: myPlayer.user_id,
            instance_id: instanceId,
          }),
        });

        if (!response.ok) {
          autoEndTurnInFlightRef.current = false;
          return;
        }

        const data = await response.json();
        turnStartMsRef.current = Date.now();
        dispatch(
          setActiveUser({
            instanceId,
            active_user: data.active_user,
            turnNumber: data.turn_number,
            energy: data.energy,
          }),
        );
      } catch {
        autoEndTurnInFlightRef.current = false;
      }
    };

    requestAutoEndTurn();
  }, [dispatch, instanceId, isMyTurn, myPlayer, turnSecsLeft, user]);

  const disconnectedPlayers = Object.entries(disconnectedDeadlines)
    .map(([id, deadline]) => {
      const userId = Number(id);
      const remainingMs = Math.max(0, deadline - nowMs);
      const remainingSec = Math.ceil(remainingMs / 1000);
      const minutes = Math.floor(remainingSec / 60);
      const seconds = remainingSec % 60;
      const playerName =
        state.players.find((p) => p.user_id === userId)?.name ??
        `Player ${userId}`;
      return {
        userId,
        playerName,
        remainingSec,
        isCritical: remainingSec <= 30,
        timerText: `${minutes}:${String(seconds).padStart(2, "0")}`,
      };
    })
    .filter((p) => p.remainingSec > 0);

  // Close the object HUD if the referenced target no longer exists in state
  useEffect(() => {
    if (!objectHUD) return;

    // Monster disappeared or died
    if (
      objectHUD.type === "monster" &&
      typeof objectHUD.x === "number" &&
      typeof objectHUD.y === "number"
    ) {
      const cell = state.grid.find(
        (c: any) => c.x === objectHUD.x && c.y === objectHUD.y,
      );
      if (!cell || !cell.monster) {
        setObjectHUD(null);
        return;
      }
      // If monster exists but HP dropped to 0 or less, close HUD to avoid constant re-renders
      if (
        cell.monster &&
        typeof cell.monster.health === "number" &&
        cell.monster.health <= 0
      ) {
        setObjectHUD(null);
        return;
      }
    }

    // Structure removed
    if (
      objectHUD.type === "structure" &&
      typeof objectHUD.x === "number" &&
      typeof objectHUD.y === "number"
    ) {
      const cell = state.grid.find(
        (c: any) => c.x === objectHUD.x && c.y === objectHUD.y,
      );
      if (!cell || !cell.structure_type) {
        setObjectHUD(null);
        return;
      }
      // Close HUD when structure health is zero or below
      if (
        typeof cell.structure_health === "number" &&
        cell.structure_health <= 0
      ) {
        setObjectHUD(null);
        return;
      }
    }

    // Player left or disconnected and removed
    if (objectHUD.type === "player" && objectHUD.userId) {
      const pl = state.players.find((p) => p.user_id === objectHUD.userId);
      if (!pl) {
        setObjectHUD(null);
        return;
      }
      // Close HUD when player HP <= 0
      if (typeof pl.health === "number" && pl.health <= 0) {
        setObjectHUD(null);
        return;
      }
    }
  }, [state.grid, state.players, objectHUD]);

  return (
    <div className={styles.container}>
      {/* HUD для выбранного объекта (монстр, постройка, игрок) */}
      {shouldShowObjectHUD && objectHUD && (
        <div
          className={`${objectHudStyles.objectHudPanel} ${
            showMiniMap && !isCompactViewport
              ? objectHudStyles.objectHudPanelBelowMinimap
              : ""
          }`}
        >
          {(() => {
            // Derive up-to-date stats from the global game state so
            // the HUD reflects damage/changes immediately.
            const base = { ...objectHUD } as any;
            if (
              objectHUD.type === "monster" &&
              typeof (objectHUD as any).x === "number"
            ) {
              const cell = state.grid.find(
                (c: any) =>
                  c.x === (objectHUD as any).x && c.y === (objectHUD as any).y,
              );
              if (cell && cell.monster) {
                base.name = cell.monster.name ?? base.name;
                base.health = cell.monster.health;
                // Preserve monster's original maxHealth so the
                // HP bar remains relative to the true maximum.
                if (typeof cell.monster.maxHealth === "number") {
                  base.maxHealth = cell.monster.maxHealth;
                } else if (typeof base.maxHealth !== "number") {
                  // Initialize maxHealth only once from current health
                  base.maxHealth = cell.monster.health;
                }
                base.attack = cell.monster.attack;
                base.defense = cell.monster.defense;
              }
            } else if (
              objectHUD.type === "structure" &&
              typeof (objectHUD as any).x === "number"
            ) {
              const cell = state.grid.find(
                (c: any) =>
                  c.x === (objectHUD as any).x && c.y === (objectHUD as any).y,
              );
              if (cell && cell.structure_type) {
                base.name =
                  base.name ||
                  (cell.structure_type === "scout_tower"
                    ? "Scout Tower"
                    : cell.structure_type === "turret"
                      ? "Turret"
                      : "Wall");
                base.health = cell.structure_health;
                base.maxHealth =
                  typeof cell.structure_health === "number"
                    ? Math.max(
                        cell.structure_health,
                        STRUCTURE_DEFAULT_MAX_HEALTH[
                          cell.structure_type as PlacementStructureType
                        ] ?? cell.structure_health,
                      )
                    : STRUCTURE_DEFAULT_MAX_HEALTH[
                        cell.structure_type as PlacementStructureType
                      ];
                base.defense = cell.structure_defense;
                base.attack = cell.structure_attack;
                base.structureType = cell.structure_type;
              }
            } else if (objectHUD.type === "player" && objectHUD.userId) {
              const pl = state.players.find(
                (p) => p.user_id === objectHUD.userId,
              );
              if (pl) {
                base.name = pl.name ?? base.name;
                base.health = pl.health;
                base.maxHealth = pl.maxHealth;
                base.energy = pl.energy;
                base.maxEnergy = pl.maxEnergy;
                base.attack = pl.attack;
                base.defense = pl.defense;
                base.groupId = (pl as any).group_id;
              }
            }

            return (
              <ObjectHUD
                {...base}
                onProfileClick={
                  base.type === "player" && base.userId
                    ? () => {
                        setProfileModalUserId(base.userId ?? null);
                        setObjectHUD(null);
                      }
                    : undefined
                }
                onClose={() => setObjectHUD(null)}
              />
            );
          })()}
        </div>
      )}
      {profileModalUserId !== null && (
        <div
          style={{
            position: "fixed",
            inset: 0,
            background: "rgba(0,0,0,0.6)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            zIndex: 1200,
          }}
          role="dialog"
          aria-modal="true"
        >
          <div
            style={{
              width: 720,
              maxWidth: "96%",
              maxHeight: "90%",
              overflow: "auto",
              background: "#111",
              border: "1px solid #444",
              padding: 16,
              borderRadius: 8,
            }}
          >
            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
                marginBottom: 8,
              }}
            >
              <h3 style={{ margin: 0 }}>
                {profileModalData?.player?.name ??
                  `Player ${profileModalUserId}`}
              </h3>
              <button
                onClick={() => setProfileModalUserId(null)}
                style={{
                  background: "transparent",
                  color: "#fff",
                  border: "1px solid #444",
                  padding: "6px 10px",
                  borderRadius: 4,
                }}
              >
                Close
              </button>
            </div>

            {profileModalLoading && <div>Loading...</div>}
            {profileModalError && (
              <div style={{ color: "#f88" }}>{profileModalError}</div>
            )}
            {!profileModalLoading && !profileModalError && profileModalData && (
              <div style={{ display: "flex", gap: 16 }}>
                <div style={{ width: 160 }}>
                  <img
                    src={
                      profileModalData.player.image ||
                      "/ui-icons/avatar-default.png"
                    }
                    alt="avatar"
                    style={{
                      width: 160,
                      height: 160,
                      objectFit: "cover",
                      borderRadius: 8,
                    }}
                  />
                </div>
                <div style={{ flex: 1 }}>
                  <div>Level: {profileModalData.player.level}</div>
                  <div>Class: {profileModalData.player.characterType}</div>
                  <div>
                    Experience: {profileModalData.player.experience} /{" "}
                    {profileModalData.player.maxExperience}
                  </div>
                  <hr
                    style={{
                      borderColor: "#333",
                      margin: "8px 0",
                    }}
                  />
                  <div>Attack: {profileModalData.player.attack}</div>
                  <div>Defense: {profileModalData.player.defense}</div>

                  {user?.id !== profileModalUserId && (
                    <div style={{ marginTop: 12 }}>
                      {!profileModalData.isFriend ? (
                        profileModalOutgoingSent ||
                        profileModalData?.friendRelation === "outgoing" ? (
                          <span
                            style={{
                              color: "#9f9",
                            }}
                          >
                            Friend request sent
                          </span>
                        ) : (
                          <button
                            onClick={async () => {
                              if (!user?.token) return;
                              setProfileModalFriendLoading(true);
                              try {
                                const res = await fetch(
                                  `${API_BASE}/game/friends/add`,
                                  {
                                    method: "POST",
                                    headers: {
                                      "Content-Type": "application/json",
                                      Authorization: `Bearer ${user?.token}`,
                                    },
                                    body: JSON.stringify({
                                      friendUserId: profileModalUserId,
                                    }),
                                  },
                                );
                                if (!res.ok) {
                                  const t = await res.text().catch(() => null);
                                  alert(t || "Failed to send request");
                                } else {
                                  // Mark as outgoing request locally
                                  setProfileModalData((d: any) => ({
                                    ...d,
                                    isFriend: false,
                                    friendRelation: "outgoing",
                                  }));
                                  setProfileModalOutgoingSent(true);
                                  try {
                                    const refetch = await fetch(
                                      `${API_BASE}/game/profile/${profileModalUserId}`,
                                      {
                                        headers: {
                                          Authorization: `Bearer ${user?.token}`,
                                        },
                                        cache: "no-store",
                                      },
                                    );
                                    if (refetch.ok) {
                                      const pdata = await refetch.json();
                                      setProfileModalData(pdata.data ?? pdata);
                                    }
                                  } catch (e) {
                                    // ignore refetch errors; local state already set
                                  }
                                }
                              } catch (e) {
                                console.error(e);
                              } finally {
                                setProfileModalFriendLoading(false);
                              }
                            }}
                            disabled={profileModalFriendLoading}
                            style={{
                              padding: "8px 12px",
                              borderRadius: 6,
                            }}
                          >
                            {profileModalFriendLoading
                              ? "Sending..."
                              : "Add Friend"}
                          </button>
                        )
                      ) : (
                        <span
                          style={{
                            color: "#9f9",
                          }}
                        >
                          Friends
                        </span>
                      )}
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        </div>
      )}
      {disconnectedPlayers.length > 0 && showDisconnectPanel && (
        <div
          className={styles.disconnectBanner}
          role="status"
          aria-live="polite"
        >
          <div
            style={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "flex-start",
              gap: 8,
            }}
          >
            <div style={{ flex: 1 }}>
              <div className={styles.disconnectTitle}>
                Player disconnected: 3:00 to reconnect
              </div>
              <div className={styles.disconnectHint}>
                If the timer runs out, the player will die from a lightning
                strike.
              </div>
            </div>
            <button
              type="button"
              className={styles.disconnectCloseButton}
              onClick={() => setShowDisconnectPanel(false)}
              aria-label="Collapse disconnect notification"
              title="Collapse"
            >
              ▾
            </button>
          </div>
          {disconnectedPlayers.map((p) => (
            <div
              key={p.userId}
              className={`${styles.disconnectRow} ${p.isCritical ? styles.disconnectRowCritical : ""}`}
            >
              <span>{p.playerName}</span>
              <span
                className={`${styles.disconnectTimer} ${p.isCritical ? styles.disconnectTimerCritical : ""}`}
              >
                {p.timerText}
              </span>
            </div>
          ))}
        </div>
      )}

      {disconnectedPlayers.length > 0 && !showDisconnectPanel && (
        <button
          type="button"
          className={styles.disconnectMinimized}
          onClick={() => setShowDisconnectPanel(true)}
          aria-label="Show disconnect notification"
          title="Show"
        >
          Player disconnects ▸
        </button>
      )}
      {myPlayer && (
        <PlayerHUD
          health={myPlayer.health}
          maxHealth={myPlayer.maxHealth}
          energy={myPlayer.energy}
          maxEnergy={myPlayer.maxEnergy}
          isRanged={myPlayer.isRanged}
          attackRange={myPlayer.attackRange}
          groupId={myPlayer.group_id}
        />
      )}
      <ObjectiveTracker
        player={myPlayer}
        players={state.players}
        grid={state.grid}
        mode={state.mode}
        questArtifactId={state.questArtifactId}
        isMyTurn={isMyTurn}
        isPlayerDefeated={isCurrentPlayerDefeated}
        hasEscaped={hasEscaped}
        isMatchFinished={isObjectiveMatchFinished}
      />
      {shouldShowActionLog && (
        <ActionLog
          entries={state.actionLog}
          compact={isCompactViewport}
          expanded={isCompactViewport ? isActionLogExpanded : undefined}
          onExpandedChange={handleActionLogExpandedChange}
        />
      )}


	  {shouldRenderCompactMiniMap ? (
  <>
    <MiniMap
      key={instanceId}
      grid={state.grid}
      mapWidth={state.mapWidth}
      mapHeight={state.mapHeight}
      players={state.players}
      instanceId={instanceId}
      myPlayerId={myPlayer?.user_id}
      activeUserId={state.active_user}
      sightRange={myPlayer?.sightRange ?? 3}
      compact
      viewportCells={minimapViewportCells}
      cameraCenterPosition={minimapFocusPoint ?? myPlayer?.position}
      onCellPing={handleMiniMapPing}
      onCollapse={handleMiniMapVisibilityToggle}
      panelClassName={`${styles.minimapPanelShell} ${
        showMiniMap
          ? styles.minimapPanelShellOpen
          : styles.minimapPanelShellClosed
      }`}
    />

    {!objectHUD && (
      <button
        type="button"
        className={`${styles.minimapToggleButton} ${styles.minimapToggleButtonCompact} ${
          isCompactActionLogVisible ? styles.minimapToggleButtonWithLog : ""
        } ${
          isCompactActionLogVisible && isActionLogExpanded
            ? styles.minimapToggleButtonLogOpen
            : ""
        } ${
          showMiniMap ? styles.minimapToggleButtonHidden : ""
        }`}
        onClick={handleMiniMapVisibilityToggle}
        aria-label="Open minimap"
        title="Open minimap"
      >
        Minimap
      </button>
    )}
  </>
) : showMiniMap ? (
  <MiniMap
    key={instanceId}
    grid={state.grid}
    mapWidth={state.mapWidth}
    mapHeight={state.mapHeight}
    players={state.players}
    instanceId={instanceId}
    myPlayerId={myPlayer?.user_id}
    activeUserId={state.active_user}
    sightRange={myPlayer?.sightRange ?? 3}
    compact={false}
    viewportCells={minimapViewportCells}
    cameraCenterPosition={minimapFocusPoint ?? myPlayer?.position}
    onCellPing={handleMiniMapPing}
    onCollapse={handleMiniMapVisibilityToggle}
  />
) : (
  <button
    type="button"
    className={`${styles.minimapToggleButton} ${
      objectHUD ? styles.minimapToggleButtonWithHud : ""
    }`}
    onClick={handleMiniMapVisibilityToggle}
    aria-label="Open minimap"
    title="Open minimap"
  >
    Minimap
  </button>
)}

      <div
        className={`${styles.turnStatusFloating} ${isMyTurn ? styles.turnStatusFloatingActive : styles.turnStatusFloatingWaiting}`}
      >
        {isMyTurn ? "YOUR TURN" : "WAITING FOR TURN"}
      </div>
      {placementMode && (
        <div className={styles.turnStatusFloating}>
          Build mode: select an adjacent cell to
          {placementMode.structureType === "scout_tower"
            ? " tower"
            : placementMode.structureType === "turret"
              ? " turret"
              : " wall"}
        </div>
      )}
      {/* Battle/mode button moved to LobbyHeader (game menu) */}
      {/* Scrolls moved into Inventory — separate panel removed */}
      <div className={styles.mapContainer}>
        {myPlayer ? (
          <MapWithCamera
            instanceId={instanceId}
            tileSize={mapViewport.tileSize}
            viewportWidth={mapViewport.width}
            viewportHeight={mapViewport.height}
            myPlayer={myPlayer}
            focusPoint={minimapFocusPoint}
            pingPoint={minimapPingPoint}
            onCellClick={handleMapCellClick}
            onPlayerClick={handleMapPlayerClick}
          />
        ) : (
          <p className={styles.mapLoading}>Loading map...</p>
        )}
      </div>
      <div className={styles.controlsContainer}>
        {isMyTurn ? (
          <>
            <div className={styles.turnPrompt}>
              <span className={styles.turnPromptBadge}>YOUR TURN</span>
              <span className={styles.turnPromptText}>
                Choose an action and end your turn
              </span>
            </div>
            <Controls onMove={handleMoveOrAttack} onAction={handleAction} />
            <TurnIndicator />
            <div className={styles.endTurnInlineDesktop}>
              <EndTurnButton
                playerId={myPlayer?.user_id!}
                instanceId={instanceId}
                onTurnEnded={handleTurnEnded}
              />
            </div>
            <button
              type="button"
              className={`${styles.inventoryDockButton} ${showInventory ? styles.inventoryFabActive : ""}`}
              onClick={handleInventoryToggle}
              aria-label={showInventory ? "Close inventory" : "Open inventory"}
              title={showInventory ? "Close inventory" : "Open inventory"}
            >
              <img
                src="/ui-icons/backpack.png"
                alt="Inventory"
                className={styles.inventoryFabIcon}
                draggable={false}
              />
            </button>
          </>
        ) : (
          <div className={styles.waitingOverlay}>
            <div className={`${styles.turnPrompt} ${styles.turnPromptWaiting}`}>
              <span
                className={`${styles.turnPromptBadge} ${styles.turnPromptBadgeWaiting}`}
              >
                WAITING FOR TURN
              </span>
              <span
                className={`${styles.turnPromptText} ${styles.turnPromptTextWaiting}`}
              >
                {waitingTurnInfo
                  ? `Now: ${waitingTurnInfo.activePlayerName}`
                  : "It's another player's turn"}
              </span>
            </div>
            {waitingTurnInfo && (
              <>
                <div className={styles.waitingEta}>
                  <span className={styles.waitingEtaLabel}>Your turn in</span>
                  <span className={styles.waitingEtaValue}>
                    {waitingTurnInfo.etaText}
                  </span>
                </div>
                <div className={styles.waitingQueue}>
                  {waitingTurnInfo.playersAhead > 0
                    ? `${waitingTurnInfo.playersAhead} player${waitingTurnInfo.playersAhead === 1 ? "" : "s"} before you`
                    : "You're next after this turn"}
                </div>
              </>
            )}
            <TurnIndicator />
            <div
              className={`${styles.turnTimer} ${styles.turnTimerInline} ${isTurnWarning ? styles.turnTimerWarn : ""}`}
            >
              {turnTimerText}
            </div>
            <button
              type="button"
              className={`${styles.inventoryDockButton} ${showInventory ? styles.inventoryFabActive : ""}`}
              onClick={handleInventoryToggle}
              aria-label={showInventory ? "Close inventory" : "Open inventory"}
              title={showInventory ? "Close inventory" : "Open inventory"}
            >
              <img
                src="/ui-icons/backpack.png"
                alt="Inventory"
                className={styles.inventoryFabIcon}
                draggable={false}
              />
            </button>
          </div>
        )}
      </div>
      {isMyTurn && (
        <div className={styles.turnActionDock}>
          <div
            className={`${styles.turnTimer} ${styles.turnTimerDock} ${isTurnWarning ? styles.turnTimerWarn : ""}`}
          >
            {turnTimerText}
          </div>
          <EndTurnButton
            playerId={myPlayer?.user_id!}
            instanceId={instanceId}
            onTurnEnded={handleTurnEnded}
          />
        </div>
      )}
      {showInventory && (
        <Inventory
          onBlueprintPlacementStart={handleBlueprintPlacementStart}
          onRequestClose={() => setShowInventory(false)}
        />
      )}
      {showQuestAlert && (
        <QuestArtifactAlert
          artifactId={state.questArtifactId}
          name={state.questArtifactName}
          image={state.questArtifactImage}
          description={state.questArtifactDescription}
          onClose={() => setShowQuestAlert(false)}
        />
      )}
      {showQuestFoundAlert && state.questFoundNotification && (
        <QuestArtifactAlert
          artifactId={state.questArtifactId}
          name={state.questArtifactName}
          image={state.questArtifactImage}
          description={state.questArtifactDescription}
          badgeText="Event"
          hintText={state.questFoundNotification.message}
          confirmLabel={questFoundConfirmLabel}
          onClose={() => {
            if (
              isDeathNotification ||
              (isPortalExitNotification && canOpenStats)
            ) {
              router.replace("/game/stats");
            }
            setShowQuestFoundAlert(false);
            dispatch(setQuestFoundNotification(null));
          }}
        />
      )}
      {showTurnModal && (
        <div
          className={styles.turnModalOverlay}
          onClick={() => setShowTurnModal(false)}
        >
          <div className={styles.turnModalCard}>
            <span className={styles.turnModalIcon}>⚔️</span>
            <span className={styles.turnModalTitle}>YOUR TURN</span>
            <span className={styles.turnModalSub}>Choose an action</span>
          </div>
        </div>
      )}
    </div>
  );
}
