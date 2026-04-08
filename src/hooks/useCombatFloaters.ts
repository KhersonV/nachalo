// =========================================
// src/hooks/useCombatFloaters.ts
// =========================================
// Отслеживает изменения HP игроков и генерирует:
//  - floaters: всплывающие числа урона / лечения
//  - flashes:  кратковременная вспышка на клетке
// Не требует изменений в Redux или бэкенде.

import { useEffect, useRef, useState } from "react";
import type { Cell, PlayerState } from "@/types/GameTypes";

export type CombatFloater = {
    id: string;
    x: number;
    y: number;
    value: number;
    isHeal: boolean;
};

export type CombatFlash = {
    id: string;
    x: number;
    y: number;
};

const LIFETIME_MS = 1100;

export function useCombatFloaters(
    players: PlayerState[],
    grid: Cell[],
): {
    floaters: CombatFloater[];
    flashes: CombatFlash[];
} {
    const [floaters, setFloaters] = useState<CombatFloater[]>([]);
    const [flashes, setFlashes] = useState<CombatFlash[]>([]);
    const prevPlayerHealthRef = useRef<Map<number, number>>(new Map());
    const prevMonsterHealthRef = useRef<Map<string, number>>(new Map());
    const prevStructureHealthRef = useRef<Map<string, number>>(new Map());

    useEffect(() => {
        const prevPlayer = prevPlayerHealthRef.current;
        const prevMonster = prevMonsterHealthRef.current;
        const newFloaters: CombatFloater[] = [];
        const newFlashes: CombatFlash[] = [];
        const now = Date.now();

        for (const player of players) {
            const prevHp = prevPlayer.get(player.user_id);
            if (prevHp !== undefined && prevHp !== player.health) {
                const delta = player.health - prevHp;
                const uid = `${player.user_id}-${now}-${Math.random().toString(36).slice(2)}`;
                newFloaters.push({
                    id: uid,
                    x: player.position.x,
                    y: player.position.y,
                    value: Math.abs(delta),
                    isHeal: delta > 0,
                });
                if (delta < 0) {
                    newFlashes.push({
                        id: `fl-${uid}`,
                        x: player.position.x,
                        y: player.position.y,
                    });
                }
            }
            prevPlayer.set(player.user_id, player.health);
        }

        const nextMonsterHealth = new Map<string, number>();
        const nextStructureHealth = new Map<string, number>();
        for (const cell of grid) {
            if (!cell.monster) continue;
            const monsterKey = getMonsterKey(cell);
            nextMonsterHealth.set(monsterKey, cell.monster.health);

            const prevHp = prevMonster.get(monsterKey);
            if (prevHp === undefined || prevHp === cell.monster.health)
                continue;

            const delta = cell.monster.health - prevHp;
            const uid = `${monsterKey}-${now}-${Math.random().toString(36).slice(2)}`;
            newFloaters.push({
                id: uid,
                x: cell.x,
                y: cell.y,
                value: Math.abs(delta),
                isHeal: delta > 0,
            });

            if (delta < 0) {
                newFlashes.push({
                    id: `fl-${uid}`,
                    x: cell.x,
                    y: cell.y,
                });
            }
        }

        prevMonsterHealthRef.current = nextMonsterHealth;

        // Structures: track structure health changes (e.g. turret)
        for (const cell of grid) {
            if (!cell.structure_type) continue;
            const key = `s-${cell.x}-${cell.y}`;
            if (typeof cell.structure_health === "number") {
                nextStructureHealth.set(key, cell.structure_health);

                const prevHp = prevStructureHealthRef.current.get(key);
                if (prevHp !== undefined && prevHp !== cell.structure_health) {
                    const delta = cell.structure_health - prevHp;
                    const uid = `${key}-${now}-${Math.random().toString(36).slice(2)}`;
                    newFloaters.push({
                        id: uid,
                        x: cell.x,
                        y: cell.y,
                        value: Math.abs(delta),
                        isHeal: delta > 0,
                    });
                    if (delta < 0) {
                        newFlashes.push({
                            id: `fl-${uid}`,
                            x: cell.x,
                            y: cell.y,
                        });
                    }
                }
            }
        }
        prevStructureHealthRef.current = nextStructureHealth;

        if (newFloaters.length === 0) return;

        const floaterIds = new Set(newFloaters.map((f) => f.id));
        const flashIds = new Set(newFlashes.map((f) => f.id));

        setFloaters((f) => [...f, ...newFloaters]);
        setFlashes((f) => [...f, ...newFlashes]);

        const t = setTimeout(() => {
            setFloaters((f) => f.filter((fl) => !floaterIds.has(fl.id)));
            setFlashes((f) => f.filter((fl) => !flashIds.has(fl.id)));
        }, LIFETIME_MS);

        return () => clearTimeout(t);
    }, [players, grid]);

    return { floaters, flashes };
}

function getMonsterKey(cell: Cell): string {
    if (cell.monster?.db_instance_id !== undefined) {
        return `m-${cell.monster.db_instance_id}`;
    }
    if (cell.monster?.id !== undefined) {
        return `m-${cell.monster.id}-${cell.x}-${cell.y}`;
    }
    return `c-${cell.x}-${cell.y}`;
}
