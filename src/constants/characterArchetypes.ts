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
        name: "Страж",
        title: "Линия фронта",
        description:
            "Живучий боец для тех, кто любит выдерживать урон и дожимать соперника вблизи.",
        image: "/guardian/guardian.webp",
        stats: [
            { label: "HP", value: "130" },
            { label: "Энергия", value: "90" },
            { label: "Восст. энергии/ход", value: "+10" },
            { label: "Атака", value: "9" },
            { label: "Защита", value: "8" },
            { label: "Мобильность", value: "2" },
            { label: "Ловкость", value: "2" },
            { label: "Обзор (клетки)", value: "2" },
            { label: "Тип боя", value: "Ближний" },
            { label: "Дальность атаки", value: "1" },
            { label: "Стоимость атаки (эн.)", value: "4" },
            { label: "Контратака", value: "Да" },
        ],
    },
    {
        id: "berserker",
        name: "Берсерк",
        title: "Агрессивный урон",
        description:
            "Максимум давления в короткой дистанции: высокий урон, но слабее защита и обзор.",
        image: "/berserk/berserk.webp",
        stats: [
            { label: "HP", value: "100" },
            { label: "Энергия", value: "100" },
            { label: "Восст. энергии/ход", value: "+10" },
            { label: "Атака", value: "14" },
            { label: "Защита", value: "3" },
            { label: "Мобильность", value: "3" },
            { label: "Ловкость", value: "2" },
            { label: "Обзор (клетки)", value: "2" },
            { label: "Тип боя", value: "Ближний" },
            { label: "Дальность атаки", value: "1" },
            { label: "Стоимость атаки (эн.)", value: "4" },
            { label: "Контратака", value: "Да" },
        ],
    },
    {
        id: "ranger",
        name: "Следопыт",
        title: "Дальний выстрел",
        description:
            "Мобильный дальник. Бьет на 2 клетки без контратаки, но не получает лишний обзор.",
        image: "/ranger/ranger.webp",
        stats: [
            { label: "HP", value: "92" },
            { label: "Энергия", value: "105" },
            { label: "Восст. энергии/ход", value: "+10" },
            { label: "Атака", value: "11" },
            { label: "Защита", value: "4" },
            { label: "Мобильность", value: "4" },
            { label: "Ловкость", value: "4" },
            { label: "Обзор (клетки)", value: "2" },
            { label: "Тип боя", value: "Дальний" },
            { label: "Дальность атаки", value: "2" },
            { label: "Стоимость атаки (эн.)", value: "6" },
            { label: "Контратака", value: "Нет" },
        ],
    },
    {
        id: "mystic",
        name: "Мистик",
        title: "Контроль темпа",
        description:
            "Тактический дальник. Атакует на 3 клетки без контратаки, но видит как остальные.",
        image: "/mag/mag.webp",
        stats: [
            { label: "HP", value: "95" },
            { label: "Энергия", value: "125" },
            { label: "Восст. энергии/ход", value: "+10" },
            { label: "Атака", value: "10" },
            { label: "Защита", value: "4" },
            { label: "Мобильность", value: "3" },
            { label: "Ловкость", value: "3" },
            { label: "Обзор (клетки)", value: "2" },
            { label: "Тип боя", value: "Дальний" },
            { label: "Дальность атаки", value: "3" },
            { label: "Стоимость атаки (эн.)", value: "6" },
            { label: "Контратака", value: "Нет" },
        ],
    },
];
