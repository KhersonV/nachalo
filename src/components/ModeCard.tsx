import Image from "next/image";
import type { GameMode } from "@/types";
import styles from "@/styles/ModesPage.module.css";

const MODE_ART: Record<GameMode, string> = {
    PVE: "/assets/ui/optimized/lone_hero_before_the_burning_fortress.webp",
    "1x1": "/assets/ui/optimized/moonlit_duel_arena_of_twin_fortresses.webp",
    "1x2": "/assets/ui/optimized/a_hero_against_two_dark_lords.webp",
    "2x2": "/assets/ui/optimized/cinematic_2v2_duel_on_a_ruined_bridge.webp",
    "3x3": "/assets/ui/optimized/siege_of_the_moonlit_fortress.webp",
    "5x5": "/assets/ui/optimized/citadel_siege_beneath_stormy_skies.webp",
};

type Props = {
    mode: GameMode;
    selected: boolean;
    disabled: boolean;
    matching: boolean;
    queuedPlayers: number;
    requiredPlayers: number;
    partySize: number;
    onSelect: () => void;
};

export default function ModeCard({ mode, selected, disabled, matching, queuedPlayers, requiredPlayers, partySize, onSelect }: Props) {
    const label = mode === "PVE" ? "PvE Solo" : `PvP ${mode.replace("x", "×")}`;
    return <button type="button" className={styles.modeCard} aria-pressed={selected} aria-disabled={disabled} onClick={() => { if (!disabled) onSelect(); }}>
        <Image src={MODE_ART[mode]} alt="" fill sizes="(max-width: 700px) 100vw, (max-width: 1200px) 50vw, 33vw" className={styles.modeImage} />
        <span className={styles.modeShade} aria-hidden="true" />
        <span className={styles.modeContent}>
            <span className={styles.modeTopline}>{mode === "PVE" ? "SOLO EXPEDITION" : "PLAYER VERSUS PLAYER"}{matching && <span className={styles.matchingTag}>Searching</span>}</span>
            <strong className={styles.modeName}>{label}</strong>
            <span className={styles.modeMeta}>{queuedPlayers} in queue <span aria-hidden="true">·</span> {requiredPlayers} needed</span>
            {disabled && <span className={styles.modeUnavailable}>Party of {partySize} exceeds team size</span>}
        </span>
    </button>;
}
