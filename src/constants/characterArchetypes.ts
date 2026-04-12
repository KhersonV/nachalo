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
            "A hardy frontline fighter with zone control and a free counterattack.",
        image: "/guardian/guardian.webp",
        stats: [
            { label: "HP", value: "130" },
            { label: "Energy", value: "90" },
            { label: "Energy regen/turn", value: "+10" },
            { label: "Attack", value: "9" },
            { label: "Defense", value: "8" },
            { label: "Mobility", value: "2" },
            { label: "Move cost (energy)", value: "4" },
            { label: "Agility", value: "2" },
            { label: "Sight (tiles)", value: "2" },
            { label: "Combat type", value: "Melee" },
            { label: "Attack range", value: "1" },
            { label: "Attack cost (energy)", value: "6" },
            { label: "Counterattack", value: "Free" },
        ],
    },
    {
        id: "berserker",
        name: "Berserker",
        title: "High Damage",
        description:
            "Maximum pressure at close range: bonus damage versus wounded targets and a frenzy follow-up after counterattacks.",
        image: "/berserk/berserk.webp",
        stats: [
            { label: "HP", value: "100" },
            { label: "Energy", value: "100" },
            { label: "Energy regen/turn", value: "+10" },
            { label: "Attack", value: "14" },
            { label: "Defense", value: "3" },
            { label: "Mobility", value: "3" },
            { label: "Move cost (energy)", value: "3" },
            { label: "Agility", value: "2" },
            { label: "Sight (tiles)", value: "2" },
            { label: "Combat type", value: "Melee" },
            { label: "Attack range", value: "1" },
            { label: "Attack cost (energy)", value: "6" },
            { label: "Counterattack", value: "2 energy" },
        ],
    },
    {
        id: "ranger",
        name: "Ranger",
        title: "Long Shot",
        description:
            "A mobile marksman who stacks Armor Break from ranged hits and converts max stacks into push pressure.",
        image: "/ranger/ranger.webp",
        stats: [
            { label: "HP", value: "92" },
            { label: "Energy", value: "105" },
            { label: "Energy regen/turn", value: "+11" },
            { label: "Attack", value: "11" },
            { label: "Defense", value: "4" },
            { label: "Mobility", value: "4" },
            { label: "Move cost (energy)", value: "3" },
            { label: "Agility", value: "4" },
            { label: "Sight (tiles)", value: "2" },
            { label: "Combat type", value: "Ranged" },
            { label: "Attack range", value: "2" },
            { label: "Attack cost (energy)", value: "8" },
            { label: "Counterattack", value: "2 energy in melee" },
        ],
    },
    {
        id: "mystic",
        name: "Mystic",
        title: "Tempo Control",
        description:
            "A tactical ranged fighter that drains enemy energy on hit and sustains its own tempo.",
        image: "/mag/mag.webp",
        stats: [
            { label: "HP", value: "95" },
            { label: "Energy", value: "125" },
            { label: "Energy regen/turn", value: "+13" },
            { label: "Attack", value: "10" },
            { label: "Defense", value: "4" },
            { label: "Mobility", value: "3" },
            { label: "Move cost (energy)", value: "3" },
            { label: "Agility", value: "3" },
            { label: "Sight (tiles)", value: "2" },
            { label: "Combat type", value: "Ranged" },
            { label: "Attack range", value: "3" },
            { label: "Attack cost (energy)", value: "8" },
            { label: "Counterattack", value: "2 energy in melee" },
        ],
    },
];
