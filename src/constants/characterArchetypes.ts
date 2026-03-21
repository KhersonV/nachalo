export type CharacterArchetype = {
    id: "guardian" | "berserker" | "ranger" | "mystic";
    name: string;
    title: string;
    description: string;
    image: string;
    stats: Array<{ label: string; value: string }>;
};

export const characterArchetypes: CharacterArchetype[] = [
    {
        id: "guardian",
        name: "Guardian",
        title: "Frontline",
        description:
            "A hardy fighter for those who like to soak damage and finish opponents up close.",
        image: "/guardian/guardian.webp",
        stats: [
            { label: "HP", value: "130" },
            { label: "Energy", value: "90" },
            { label: "Energy regen/turn", value: "+10" },
            { label: "Attack", value: "9" },
            { label: "Defense", value: "8" },
            { label: "Mobility", value: "2" },
            { label: "Agility", value: "2" },
            { label: "Sight (tiles)", value: "2" },
            { label: "Combat type", value: "Melee" },
            { label: "Attack range", value: "1" },
            { label: "Attack cost (energy)", value: "4" },
            { label: "Counterattack", value: "Yes" },
        ],
    },
    {
        id: "berserker",
        name: "Berserker",
        title: "High Damage",
        description:
            "Maximum pressure at close range: high damage but weaker defense and sight.",
        image: "/berserk/berserk.webp",
        stats: [
            { label: "HP", value: "100" },
            { label: "Energy", value: "100" },
            { label: "Energy regen/turn", value: "+10" },
            { label: "Attack", value: "14" },
            { label: "Defense", value: "3" },
            { label: "Mobility", value: "3" },
            { label: "Agility", value: "2" },
            { label: "Sight (tiles)", value: "2" },
            { label: "Combat type", value: "Melee" },
            { label: "Attack range", value: "1" },
            { label: "Attack cost (energy)", value: "4" },
            { label: "Counterattack", value: "Yes" },
        ],
    },
    {
        id: "ranger",
        name: "Ranger",
        title: "Long Shot",
        description:
            "A mobile marksman. Hits at 2 tiles without counterattack, but doesn't gain extra sight.",
        image: "/ranger/ranger.webp",
        stats: [
            { label: "HP", value: "92" },
            { label: "Energy", value: "105" },
            { label: "Energy regen/turn", value: "+10" },
            { label: "Attack", value: "11" },
            { label: "Defense", value: "4" },
            { label: "Mobility", value: "4" },
            { label: "Agility", value: "4" },
            { label: "Sight (tiles)", value: "2" },
            { label: "Combat type", value: "Ranged" },
            { label: "Attack range", value: "2" },
            { label: "Attack cost (energy)", value: "6" },
            { label: "Counterattack", value: "No" },
        ],
    },
    {
        id: "mystic",
        name: "Mystic",
        title: "Tempo Control",
        description:
            "A tactical ranged fighter. Attacks at 3 tiles without counterattack, with balanced sight.",
        image: "/mag/mag.webp",
        stats: [
            { label: "HP", value: "95" },
            { label: "Energy", value: "125" },
            { label: "Energy regen/turn", value: "+10" },
            { label: "Attack", value: "10" },
            { label: "Defense", value: "4" },
            { label: "Mobility", value: "3" },
            { label: "Agility", value: "3" },
            { label: "Sight (tiles)", value: "2" },
            { label: "Combat type", value: "Ranged" },
            { label: "Attack range", value: "3" },
            { label: "Attack cost (energy)", value: "6" },
            { label: "Counterattack", value: "No" },
        ],
    },
];
