
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//************************************************* src/components/ArtifactTransferDialog.tsx *********************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************
//*****************************************************************************************************************************


import React from "react";
import { InventoryItem } from "../logic/types";

type ArtifactTransferDialogProps = {
  // Здесь у нас Record<string, InventoryItem>
  artifacts: Record<string, InventoryItem>;
  onSelectArtifact: (artifactKey: string) => void;
  onCancel: () => void;
};

export default function ArtifactTransferDialog({
  artifacts,
  onSelectArtifact,
  onCancel,
}: ArtifactTransferDialogProps) {
  const artifactEntries = Object.entries(artifacts);

  return (
    <div className="artifact-dialog">
      <h3>Выберите артефакт для передачи</h3>
      <ul>
        {artifactEntries.map(([key, item]) => (
          <li key={key} onClick={() => onSelectArtifact(key)}>
            <img
              src={item.image}
              alt={item.name || key}
              style={{ width: "50px" }}
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
