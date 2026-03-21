import React, { useMemo } from "react";
import { Cell, PlayerState } from "@/types/GameTypes";
import styles from "@/styles/Map.module.css";

type Visibility = "visible" | "explored" | "unknown";

interface MapCellProps {
    cell: Cell;
    playerInCell: PlayerState | null;
    visibility: Visibility;
    tileSize: number;
    isCurrentPlayerCell?: boolean;
    onClick?: (cell: Cell) => void;
}

function MapCell({
    cell,
    playerInCell,
    visibility,
    tileSize,
    isCurrentPlayerCell = false,
    onClick,
}: MapCellProps) {
    const isVisible = visibility === "visible";
    const isInteractive = isVisible && !!onClick;

    const imageStyle = useMemo<React.CSSProperties>(
        () => ({
            width: "100%",
            height: "100%",
            objectFit: "cover",
        }),
        [],
    );

    const tileStyle = useMemo<React.CSSProperties>(() => {
        return {
            width: tileSize,
            height: tileSize,
            background: getTileBackground(cell),
            pointerEvents: isVisible ? "auto" : "none",
            cursor: isInteractive ? "pointer" : "default",
        };
    }, [cell, tileSize, isVisible, isInteractive]);

    const tileClassName = useMemo(
        () =>
            [
                styles.cell,
                isVisible
                    ? styles.visible
                    : visibility === "explored"
                      ? styles.explored
                      : styles.unknown,
                isInteractive ? styles.interactive : "",
                isCurrentPlayerCell ? styles.currentPlayerCell : "",
                playerInCell ? styles.hasPlayer : "",
            ]
                .filter(Boolean)
                .join(" "),
        [visibility, isVisible, isInteractive, isCurrentPlayerCell, playerInCell],
    );

    const cellContent = useMemo(() => {
        if (!isVisible) return null;

        if (cell.structure_type) {
            const symbol =
                cell.structure_type === "scout_tower"
                    ? "🗼"
                    : cell.structure_type === "turret"
                      ? "🔫"
                      : "🧱";

            return (
                <span className={styles.symbol}>
                    {cell.is_under_construction ? "🚧" : symbol}
                </span>
            );
        }

        if (cell.monster?.image) {
            return (
                <img
                    src={cell.monster.image}
                    alt="monster"
                    className={styles.image}
                    style={imageStyle}
                />
            );
        }

        if (cell.resource?.image) {
            return (
                <img
                    src={cell.resource.image}
                    alt="resource"
                    className={styles.image}
                    style={imageStyle}
                />
            );
        }

        if (cell.barbel?.image) {
            return (
                <img
                    src={cell.barbel.image}
                    alt="barrel"
                    className={styles.image}
                    style={imageStyle}
                />
            );
        }

        if (cell.isPortal) {
            return (
                <img
                    src="/portal.png"
                    alt="portal"
                    className={styles.image}
                    style={imageStyle}
                />
            );
        }

        if (cell.monster) return <span className={styles.symbol}>👹</span>;
        if (cell.resource) return <span className={styles.symbol}>⛏</span>;
        if (cell.barbel) return <span className={styles.symbol}>🪵</span>;

        return null;
    }, [cell, imageStyle, isVisible]);

    const handleClick = () => {
        if (isVisible) {
            onClick?.(cell);
        }
    };

    return (
        <div
            className={tileClassName}
            style={tileStyle}
            onClick={handleClick}
        >
            {cellContent}

            {isVisible && (
                <span className={styles.coords}>{`${cell.x}:${cell.y}`}</span>
            )}
        </div>
    );
}

function getTileBackground(cell: Cell): string {
    switch (cell.tileCode) {
        case 48:
            return "linear-gradient(155deg, #9da5ad 0%, #7b858f 100%)";
        case 80:
            return "linear-gradient(155deg, #2f68bf 0%, #183a72 100%)";
        case 32:
            return "linear-gradient(155deg, #3f4954 0%, #2a3139 100%)";
        case 77:
            return "linear-gradient(155deg, #a93939 0%, #7f2727 100%)";
        case 82:
            return "linear-gradient(155deg, #2f8d64 0%, #1f6a4a 100%)";
        case 112:
            return "linear-gradient(155deg, #45c7b0 0%, #1b8f8d 100%)";
        case 66:
            return "linear-gradient(155deg, #cb8a45 0%, #8f5f2e 100%)";
        default:
            return "linear-gradient(155deg, #8a4b42 0%, #6d362f 100%)";
    }
}

export default React.memo(MapCell);
