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
            "A durable defender with aura and two reactions. Effective against Berserker.",
        reflexEffectName: getClassReflexEffectName("guardian"),
        reflexEffectDescription: getClassReflexEffectDescription("guardian"),
        image: "/guardian/guardian.webp",
        stats: [
            { label: "HP", value: "130" },
            { label: "AP", value: "3" },
            { label: "Attack", value: "13" },
            { label: "Defense", value: "9" },
            { label: "Mobility", value: "2" },
            { label: "MP", value: "2" },
            { label: "Move cost", value: "0.5 MP" },
            { label: "Reflex", value: reflexStat("guardian").reflex },
            { label: "Proc chance", value: reflexStat("guardian").chance },
            { label: "Sight (tiles)", value: "2" },
            { label: "Combat type", value: "Melee" },
            { label: "Attack range", value: "1" },
            { label: "Attack cost", value: "1 AP" },
            { label: "Reactions", value: "2" },
        ],
    },
    {
        id: "berserker",
        name: "Berserker",
        title: "High Damage",
        description:
            "A melee fighter with finishing damage and follow-up strikes. Effective against Ranger.",
        reflexEffectName: getClassReflexEffectName("berserker"),
        reflexEffectDescription: getClassReflexEffectDescription("berserker"),
        image: "/berserk/berserk.webp",
        stats: [
            { label: "HP", value: "100" },
            { label: "AP", value: "3" },
            { label: "Attack", value: "17" },
            { label: "Defense", value: "4" },
            { label: "Mobility", value: "3" },
            { label: "MP", value: "3" },
            { label: "Move cost", value: "0.5 MP" },
            { label: "Reflex", value: reflexStat("berserker").reflex },
            { label: "Proc chance", value: reflexStat("berserker").chance },
            { label: "Sight (tiles)", value: "2" },
            { label: "Combat type", value: "Melee" },
            { label: "Attack range", value: "1" },
            { label: "Attack cost", value: "1 AP" },
            { label: "Reactions", value: "1" },
        ],
    },
    {
        id: "ranger",
        name: "Ranger",
        title: "Long Shot",
        description:
            "A marksman with critical hits and armor break. Effective against Mystic.",
        reflexEffectName: getClassReflexEffectName("ranger"),
        reflexEffectDescription: getClassReflexEffectDescription("ranger"),
        image: "/ranger/ranger.webp",
        stats: [
            { label: "HP", value: "92" },
            { label: "AP", value: "3" },
            { label: "Attack", value: "16" },
            { label: "Defense", value: "7" },
            { label: "Mobility", value: "4" },
            { label: "MP", value: "4" },
            { label: "Move cost", value: "0.5 MP" },
            { label: "Reflex", value: reflexStat("ranger").reflex },
            { label: "Proc chance", value: reflexStat("ranger").chance },
            { label: "Sight (tiles)", value: "2" },
            { label: "Combat type", value: "Ranged" },
            { label: "Attack range", value: "2" },
            { label: "Attack cost", value: "1 AP" },
            { label: "Reactions", value: "1" },
        ],
    },
    {
        id: "mystic",
        name: "Mystic",
        title: "Tempo Control",
        description:
            "A caster with Overload and MP drain. Effective against Guardian.",
        reflexEffectName: getClassReflexEffectName("mystic"),
        reflexEffectDescription: getClassReflexEffectDescription("mystic"),
        image: "/mag/mag.webp",
        stats: [
            { label: "HP", value: "95" },
            { label: "AP", value: "3" },
            { label: "Attack", value: "18" },
            { label: "Defense", value: "5" },
            { label: "Mobility", value: "3" },
            { label: "MP", value: "3" },
            { label: "Move cost", value: "0.5 MP" },
            { label: "Reflex", value: reflexStat("mystic").reflex },
            { label: "Proc chance", value: reflexStat("mystic").chance },
            { label: "Sight (tiles)", value: "3" },
            { label: "Combat type", value: "Ranged" },
            { label: "Attack range", value: "3" },
            { label: "Attack cost", value: "1 AP" },
            { label: "Reactions", value: "1" },
        ],
    },
];
