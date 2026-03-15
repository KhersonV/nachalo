//==========================
// src/utils/toGameObject.ts
//==========================

import { GameObject } from "../components/InfoModal";
import type { Cell, PlayerState } from "../types";

export function cellToGameObject(cell: Cell): GameObject {
    if (cell.monster) {
        return {
            type: "monster",
            name: cell.monster.name,
            hp: cell.monster.health,
            aggressive: cell.monster.type === "aggressive",
            attack: cell.monster.attack,
            defense: cell.monster.defense,
        };
    }
    if (cell.resource || cell.barbel) {
        const res = cell.resource ?? cell.barbel!;
        return {
            type: "resource",
            name: res.type,
            description: res.description,
            effects: Object.entries(res.effect).map(([k, v]) => `${k}: ${v}`),
        };
    }
    if (cell.isPortal) {
        return {
            type: "portal",
            requirement: "Нужен артефакт, чтобы выйти",
        };
    }
    // простая клетка, пустота или стена — передаём координаты
    return {
        type:
            cell.tileCode === 32
                ? "empty"
                : cell.tileCode === 48
                  ? "cell"
                  : "wall",
        x: cell.x,
        y: cell.y,
    } as GameObject;
}

export function playerToGameObject(p: PlayerState): GameObject {
    return {
        type: "player",
        name: p.name,
        defense: p.defense,
        attack: p.attack,
        hp: p.health,
    };
}
