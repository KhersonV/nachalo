import {
    getBaseReflexForHero,
    getClassReflexEffectDescription,
    getClassReflexEffectName,
    getReflexProcChance,
} from "../utils/reflex";

export type CharacterArchetype = {
    id: "guardian" | "berserker" | "ranger" | "mystic";
    name: string;
    title: string;
    description: string;
    reflexEffectName: string;
    reflexEffectDescription: string;
    image: string;
    stats: Array<{ label: string; value: string }>;
};

const reflexStat = (heroClassId: CharacterArchetype["id"]) => {
    const reflex = getBaseReflexForHero(heroClassId);
    return {
        reflex: String(reflex),
        chance: `${getReflexProcChance(reflex)}%`,
    };
};

export const characterArchetypes: CharacterArchetype[] = [
    {
        id: "guardian",
        name: "Guardian",
        title: "Frontline",
        description:
            "A hardy frontline fighter with zone control and a free counterattack.",
        reflexEffectName: getClassReflexEffectName("guardian"),
        reflexEffectDescription: getClassReflexEffectDescription("guardian"),
        image: "/guardian/guardian.webp",
        stats: [
            { label: "HP", value: "130" },
            { label: "Energy", value: "90" },
            { label: "Energy regen/turn", value: "+10" },
            { label: "Attack", value: "9" },
            { label: "Defense", value: "8" },
            { label: "Mobility", value: "2" },
            { label: "Move cost (energy)", value: "4" },
            { label: "Reflex", value: reflexStat("guardian").reflex },
            { label: "Proc chance", value: reflexStat("guardian").chance },
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
        reflexEffectName: getClassReflexEffectName("berserker"),
        reflexEffectDescription: getClassReflexEffectDescription("berserker"),
        image: "/berserk/berserk.webp",
        stats: [
            { label: "HP", value: "100" },
            { label: "Energy", value: "100" },
            { label: "Energy regen/turn", value: "+10" },
            { label: "Attack", value: "14" },
            { label: "Defense", value: "3" },
            { label: "Mobility", value: "3" },
            { label: "Move cost (energy)", value: "3" },
            { label: "Reflex", value: reflexStat("berserker").reflex },
            { label: "Proc chance", value: reflexStat("berserker").chance },
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
        reflexEffectName: getClassReflexEffectName("ranger"),
        reflexEffectDescription: getClassReflexEffectDescription("ranger"),
        image: "/ranger/ranger.webp",
        stats: [
            { label: "HP", value: "92" },
            { label: "Energy", value: "105" },
            { label: "Energy regen/turn", value: "+11" },
            { label: "Attack", value: "11" },
            { label: "Defense", value: "4" },
            { label: "Mobility", value: "4" },
            { label: "Move cost (energy)", value: "3" },
            { label: "Reflex", value: reflexStat("ranger").reflex },
            { label: "Proc chance", value: reflexStat("ranger").chance },
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
        reflexEffectName: getClassReflexEffectName("mystic"),
        reflexEffectDescription: getClassReflexEffectDescription("mystic"),
        image: "/mag/mag.webp",
        stats: [
            { label: "HP", value: "95" },
            { label: "Energy", value: "125" },
            { label: "Energy regen/turn", value: "+13" },
            { label: "Attack", value: "10" },
            { label: "Defense", value: "4" },
            { label: "Mobility", value: "3" },
            { label: "Move cost (energy)", value: "3" },
            { label: "Reflex", value: reflexStat("mystic").reflex },
            { label: "Proc chance", value: reflexStat("mystic").chance },
            { label: "Sight (tiles)", value: "2" },
            { label: "Combat type", value: "Ranged" },
            { label: "Attack range", value: "3" },
            { label: "Attack cost (energy)", value: "8" },
            { label: "Counterattack", value: "2 energy in melee" },
        ],
    },
];
