
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//************************************************* src/components/ArtifactTransferDialog.tsx *********************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************

"use client";
import React from "react";
import { InventoryItem } from "../logic/types";

type ArtifactTransferDialogProps = {
  artifacts: Record<string, InventoryItem>;
  onSelectArtifact: (artifactKey: string) => void;
  onCancel: () => void;
};

export default function ArtifactTransferDialog({
  artifacts,
  onSelectArtifact,
  onCancel,
}: ArtifactTransferDialogProps) {
  // Массив: [[artifactKey, artifactItem], [artifactKey2, artifactItem2], ...]
  const artifactEntries = Object.entries(artifacts);

  // Логируем список ключей — чтобы убедиться, какие именно ключи лежат в проигравшем
  console.log("ArtifactTransferDialog => artifact keys:", artifactEntries.map(([k]) => k));

  return (
    <div className="artifact-dialog" style={{ border: "2px solid red", padding: 20 }}>
      <h3>Выберите артефакт для передачи</h3>
      <ul>
        {artifactEntries.map(([key, item]) => (
          <li
            key={key}
            onClick={() => {
              console.log("ArtifactTransferDialog => clicked on", key);
              onSelectArtifact(key);
            }}
            style={{ cursor: "pointer", border: "1px dashed #ccc", margin: 10 }}
          >
            <img
              src={item.image}
              alt={item.name || key}
              style={{ width: 50, height: 50, objectFit: "cover" }}
            />
            <p>{item.name || key}</p>
            <p>{item.description}</p>
            {item.bonus && (
              <p>
                <strong>Бонусы:</strong> {JSON.stringify(item.bonus)}
              </p>
            )}
          </li>
        ))}
      </ul>
      <button onClick={onCancel}>Отмена</button>
    </div>
  );
}
