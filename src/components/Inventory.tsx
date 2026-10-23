//==================================
// src/components/Inventory.tsx
//==================================

"use client";

import React, { useEffect, useMemo, useState } from "react";
import type { RawInventoryItem } from "../types";
import styles from "../styles/Inventory.module.css";
import type { HexGameMessages } from "@/i18n/hexGame";
import { resourceImage } from "@/utils/resourceAssets";
import { equipmentImage } from "@/utils/equipmentAssets";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";

interface InventoryProps {
  className?: string;
  onRequestClose?: () => void;
  hexInventory: Record<string, { item_count?: number; item_name?: string }>;
  equipmentLoot?: Array<{ itemInstanceId: string; name: string; rarity: string; image: string }>;
  onHexUseItem?: (itemType: "resource" | "scroll", itemId: number) => Promise<string>;
  onHexBlueprintPlacementStart?: (inventoryKey: string) => void;
  hexBusy?: boolean;
  hexLabels?: HexGameMessages;
  questArtifactId?: number;
}

const isConsumableResource = (item: RawInventoryItem): boolean => {
  if (item.item_type !== "resource") return false;
  const effect = item.effect || {};
  if (typeof effect.health === "number" || typeof effect.energy === "number") {
    return true;
  }

  const normalizedName = String(item.name || "")
    .trim()
    .toLowerCase();
  return normalizedName === "food" || normalizedName === "water";
};

const Inventory: React.FC<InventoryProps> = ({
  className,
  onRequestClose,
  hexInventory,
  equipmentLoot = [],
  onHexUseItem,
  onHexBlueprintPlacementStart,
  hexBusy,
  hexLabels,
  questArtifactId,
}) => {
  const [feedback, setFeedback] = useState("");
  const { language } = useGameLanguage();
  const t = (source: string) => uiText(language, source);

  useEffect(() => {
    if (!feedback) return;
    const timer = window.setTimeout(() => {
      setFeedback("");
    }, 4000);
    return () => window.clearTimeout(timer);
  }, [feedback]);


  // Images are provided by the server in inventory entries; use them directly.

  const normalize = (it: any, keyHint?: string): RawInventoryItem => {
    let hintType: "resource" | "artifact" | undefined;
    let hintId: number | undefined;
    if (keyHint) {
      const [t, id] = keyHint.split("_");
      if (
        (t === "resource" || t === "artifact" || t === "scroll") &&
        !isNaN(+id)
      ) {
        hintType = t as any;
        hintId = +id;
      }
    }

    const explicitItemType =
      it.item_type === "resource" ||
      it.item_type === "artifact" ||
      it.item_type === "scroll"
        ? it.item_type
        : undefined;
    const item_type = (explicitItemType || hintType || "resource") as
      | "resource"
      | "artifact"
      | "scroll";
    const item_id = it.item_id || it.id || hintId || 0;
    const name = it.name || it.item_name || item_type + "_" + item_id;
    const item_count = it.item_count || it.count || 1;
    const image = it.image || it.image_url || "";
    const description = it.description || it.item_description || "";
    const canonicalInventoryKey =
      explicitItemType && item_id > 0 ? `${item_type}_${item_id}` : keyHint;

    return {
      item_type,
      item_id,
      name,
      item_count,
      image,
      description,
      bonus: it.bonus,
      effect: it.effect,
      inventory_key: canonicalInventoryKey,
    };
  };

  const rawItems: RawInventoryItem[] = useMemo(() => {
    let parsed: any;
    try {
      parsed = hexInventory;
    } catch {
      return [];
    }

    let flat: any[] = [];
    if (Array.isArray(parsed)) {
      flat = parsed;
    } else if (parsed.resources || parsed.artifacts) {
      flat = [
        ...(parsed.resources ? Object.entries(parsed.resources) : []),
        ...(parsed.artifacts ? Object.entries(parsed.artifacts) : []),
      ].map(([key, val]) => ({ val, key }));
    } else {
      flat = Object.entries(parsed).map(([key, val]) => ({ val, key }));
    }

    return flat.map((entry) => {
      if ((entry as any).val !== undefined) {
        return normalize((entry as any).val, (entry as any).key);
      } else {
        return normalize(entry);
      }
    });
  }, [hexInventory]);

  const { resources, artifacts } = useMemo(() => {
    return rawItems.reduce<{
      resources: Record<string, RawInventoryItem>;
      artifacts: Record<string, RawInventoryItem>;
    }>(
      (acc, it) => {
        const key = it.inventory_key || `${it.item_type}_${it.item_id}`;
        if (it.item_type === "resource" || it.item_type === "scroll")
          acc.resources[key] = it;
        else acc.artifacts[key] = it;
        return acc;
      },
      { resources: {}, artifacts: {} },
    );
  }, [rawItems]);

  const handleUseItem = async (key: string, item: RawInventoryItem) => {
    if (item.item_count <= 0) return;
    if (key.startsWith("blueprint_")) {
      onHexBlueprintPlacementStart?.(key);
      onRequestClose?.();
      return;
    }
    const isScroll = key.startsWith("scroll_");
    if (!onHexUseItem || (!isScroll && !isConsumableResource(item))) return;
    try {
      setFeedback(await onHexUseItem(isScroll ? "scroll" : "resource", item.item_id));
    } catch (reason) { setFeedback(String(reason)); }
  };

  const renderItems = (
    section: "resources" | "artifacts",
    items: Record<string, RawInventoryItem>,
  ) =>
    Object.entries(items)
      .filter(([, item]) => item.item_count > 0)
      .map(([key, item]) => (
        <div key={key} className={styles.item}>
          {(() => {
            // For blueprints, prefer showing the built-structure image
            const isBlueprint = key.startsWith("blueprint_");
            const blueprintName = isBlueprint
              ? key.replace("blueprint_", "")
              : null;
            const displayImage = isBlueprint
              ? `/Forge-items/${blueprintName}.png`
              : item.item_type === "resource" ? resourceImage(item.name, item.image) ?? item.image ?? "" : item.image || "";

            return (
              <img
                src={displayImage}
                alt={item.name}
                className={styles.itemImage}
                loading="lazy"
              />
            );
          })()}
          <div className={styles.itemName}>{t(item.name)}</div>
          {section === "artifacts" && item.item_id === questArtifactId && <div className={styles.itemCount}>{hexLabels?.questArtifact ?? t("Quest artifact")}</div>}
          {section === "resources" && (
            <>
              <div className={styles.itemCount}>{hexLabels?.quantity ?? t("Qty")}: {item.item_count}</div>
              {(isConsumableResource(item) || key.startsWith("scroll_") || key.startsWith("blueprint_")) && (
                <button
                  className={styles.useButton}
                  disabled={hexBusy || (key.startsWith("blueprint_") ? !onHexBlueprintPlacementStart : !onHexUseItem)}
                  onClick={() => handleUseItem(key, item)}
                >
                  {key.startsWith("blueprint_") ? t("Place") : (hexLabels?.use ?? t("Use"))}
                </button>
              )}
            </>
          )}
        </div>
      ));

  return (
    <div className={`${styles.inventoryContainer} ${className ?? ""}`}>
      <div className={styles.sheetHeader}><h2 className={styles.title}>{hexLabels?.inventory ?? t("Inventory")}</h2>{onRequestClose && <button type="button" className={styles.sheetClose} onClick={onRequestClose} aria-label={hexLabels?.closeInventory ?? t("Close inventory")}>×</button>}</div>
      {feedback && (
        <div className={styles.feedbackRow}>
          <p className={styles.feedbackError}>{feedback}</p>
          <button
            type="button"
            className={styles.feedbackCloseButton}
            onClick={() => setFeedback("")}
            aria-label={hexLabels?.closeMessage ?? t("Close message")}
            title={hexLabels?.close ?? t("Close")}
          >
            x
          </button>
        </div>
      )}
      <div className={styles.sections}>
        <div className={styles.section}>
          <h3 className={styles.sectionTitle}>{hexLabels?.resources ?? t("Resources")}</h3>
          <p className={styles.sectionHint}>
            {hexLabels?.useLimits ?? t("Limits: food once per 2 turns, water up to 5 per 2 turns.")}
          </p>
          <div className={styles.itemsContainer}>
            {renderItems("resources", resources)}
          </div>
        </div>
        <div className={styles.section}>
          <h3 className={styles.sectionTitle}>{hexLabels?.artifacts ?? t("Artifacts")}</h3>
          <div className={styles.itemsContainer}>
            {renderItems("artifacts", artifacts)}
          </div>
        </div>
        <div className={styles.section}>
          <h3 className={styles.sectionTitle}>{hexLabels?.categories.equipment ?? t("Equipment")}</h3>
          <div className={styles.itemsContainer}>
            {equipmentLoot.map(item => <div key={item.itemInstanceId} className={styles.item}>
              <img src={equipmentImage(item.image)} alt={item.name} className={styles.itemImage} loading="lazy" />
              <div className={styles.itemName}>{t(item.name)}</div>
              <div className={styles.itemCount}>{item.rarity}</div>
            </div>)}
          </div>
        </div>
      </div>
    </div>
  );
};

export default Inventory;
