"use client";

import React from "react";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";
import UiText from "./UiText";
import { professionRankName } from "@/utils/professionProgression";
import EquipmentSetsGuide, { type EquipmentGuideSet } from "./EquipmentSetsGuide";
import type { CraftRecipe } from "./ProfessionCrafting";
import Link from "next/link";
import { useAuth } from "@/contexts/AuthContext";
import { API_BASE } from "@/utils/serviceUrls";
import { Button, Card, EmptyState, ErrorState, LoadingState, PageHeader, Panel } from "@/components/ui";
import styles from "@/styles/DropGuidePage.module.css";

type Material = {
    code: string; name: string; description: string; image: string; class: string;
    professionCode: string; professionName: string; quantity: number; chance: number;
    processing: string; tradable: boolean; resourceId: number;
};
type Monster = {
    code: string; name: string; image: string; health: number; attack: number; defense: number;
    xp: number; professionChance: number; equipmentChance: number; materials: Material[];
};
type GatheringMaterial = { code: string; name: string; description: string; minLevel: number; chance: number; image: string; tradable: boolean; resourceId: number };
type GatheringCategory = { code: string; name: string; description: string; class: string; profession: string; professionName: string; image: string; materials: GatheringMaterial[] };
type Catalog = { equipmentSets?: EquipmentGuideSet[]; recipes?: CraftRecipe[]; gathering?: GatheringCategory[]; gatheringLimit?: number; gatheringXP?: number; monsters: Monster[]; successLimit: number; baseEquipmentChance: number };
const classes = [{ id: "all", name: "Все классы" }, { id: "berserker", name: "Берсерк" }, { id: "ranger", name: "Рейнджер" }, { id: "guardian", name: "Страж" }, { id: "mystic", name: "Маг" }];
const className = (id: string) => classes.find(value => value.id === id)?.name || id;
const percent = (value: number) => `${(value * 100).toLocaleString("ru-RU", { maximumFractionDigits: 2 })}%`;

export default function DropGuidePage({ embedded = false, initialClass = "all", professionLevel }: { embedded?: boolean; initialClass?: string; professionLevel?: number }) {
    const { user } = useAuth();
    const { language } = useGameLanguage();
    const l = (source: string) => uiText(language, source);
    const [catalog, setCatalog] = React.useState<Catalog | null>(null);
    const [error, setError] = React.useState("");
    const [loading, setLoading] = React.useState(true);
    const [monsterCode, setMonsterCode] = React.useState<string | null>(null);
    const [materialCode, setMaterialCode] = React.useState<string | null>(null);
    const [filter, setFilter] = React.useState(initialClass);
    const serial = React.useRef(0);
    const detail = React.useRef<HTMLDialogElement>(null);
    const load = React.useCallback(async () => {
        if (!user?.token) return;
        const requestID = ++serial.current;
        setError("");
        try {
            const response = await fetch(`${API_BASE}/game/professions/monster-loot`, { headers: { Authorization: `Bearer ${user.token}` }, cache: "no-store" });
            if (!response.ok) throw new Error("Справочник временно недоступен");
            const result: Catalog = await response.json();
            if (requestID === serial.current) setCatalog(result);
        } catch (reason) {
            if (requestID === serial.current) setError(reason instanceof Error ? reason.message : "Не удалось загрузить справочник");
        } finally {
            if (requestID === serial.current) setLoading(false);
        }
    }, [user?.token]);
    React.useEffect(() => {
        setLoading(true);
        void load();
        const refresh = () => { void load(); };
        window.addEventListener("focus", refresh);
        return () => { ++serial.current; window.removeEventListener("focus", refresh); };
    }, [load]);
    React.useEffect(() => { setFilter(initialClass); setMaterialCode(null); }, [initialClass]);
    const Wrapper = embedded ? "section" : "main";
    const monster = catalog?.monsters.find(value => value.code === monsterCode);
    const material = monster?.materials.find(value => value.code === materialCode);
    React.useEffect(() => {
        if (material && !detail.current?.open) detail.current?.showModal();
        if (!material && detail.current?.open) detail.current.close();
    }, [material]);
    const selectMonster = (code: string | null) => { setMaterialCode(null); setMonsterCode(code); };

    const recipeLabels = (recipes: CraftRecipe[]) => recipes.map((recipe, index) => <React.Fragment key={recipe.code}>{index > 0 && ", "}<span><img src={recipe.image} alt="" width={24} height={24} loading="lazy" className={styles.recipeIcon} /> {l(recipe.name)}</span></React.Fragment>);
    const uses = (code: string) => {
        const direct = catalog?.recipes?.filter(recipe => recipe.ingredients.some(ingredient => ingredient.code === code)) || [];
        const reachable = new Set<string>([code]);
        // Follow the entire catalogue: ore → ingot → powder → consumable.
        // A visited set also keeps malformed cyclic data from looping forever.
        const pending = [code];
        while (pending.length > 0) {
            const ingredientCode = pending.pop();
            for (const recipe of catalog?.recipes || []) {
                if (!reachable.has(recipe.code) && recipe.ingredients.some(ingredient => ingredient.code === ingredientCode)) {
                    reachable.add(recipe.code);
                    pending.push(recipe.code);
                }
            }
        }
        const components = catalog?.recipes?.filter(recipe => recipe.category === "processed" && reachable.has(recipe.code) && !direct.some(value => value.code === recipe.code)) || [];
        const finished = catalog?.recipes?.filter(recipe => recipe.category !== "processed" && reachable.has(recipe.code)) || [];
        return <div><p><strong>{l("Processing and products")}: </strong>{recipeLabels(direct)}</p>{components.length > 0 && <p><strong>{l("Intermediate components")}: </strong>{recipeLabels(components)}</p>}{finished.length > 0 && <p><strong>{l("Finished products")}: </strong>{recipeLabels(finished)}</p>}</div>;
    };
    return <Wrapper className={`${styles.page} ${embedded ? styles.embedded : ""}`}><div className={styles.container}>
        {embedded ? <header><h2><UiText text="Profession materials" /></h2><p><UiText text="The selected hero class is shown first; other classes can also be inspected." /></p></header> : <PageHeader title={l("Drop guide")} description={l("Combat trophies and peaceful profession gathering in HEX PvE/PvP")} actions={<Button onClick={() => void load()}>Обновить</Button>} />}
        {loading && <LoadingState title="Загрузка справочника…" />}
        {error && <ErrorState title={error} action={<Button onClick={() => void load()}>Повторить</Button>} />}
        {catalog && <>

            <div className={styles.filters} role="group" aria-label={l("Class")}>{classes.map(value => <Button key={value.id} aria-pressed={filter === value.id} variant={filter === value.id ? "primary" : "secondary"} onClick={() => { setFilter(value.id); setMaterialCode(null); }}>{uiText(language, value.id === "all" ? "All classes" : value.id)}</Button>)}</div>
            {!embedded && <EquipmentSetsGuide sets={catalog.equipmentSets || []} recipes={catalog.recipes || []} monsters={catalog.monsters} heroClass={filter} />}
            <section className={styles.gatheringSection} aria-label={l("Peaceful professional resources")}>
                <h2><UiText text="Peaceful professional resources" /></h2>
                <p><UiText text="Gather on the object's HEX cell for 0.5 AP. The Guild must be built before the match." /></p>
                <p><UiText text="Independent peaceful gathering limit" />: {catalog.gatheringLimit} · +{catalog.gatheringXP} XP / <UiText text="successful gathering" /></p>
                <p><UiText text="Unavailable rewards are excluded and the remaining chances are normalized; gathering never gives an empty reward." /></p>
                {catalog.gathering?.filter(category => filter === "all" || category.class === filter).map(category => {
                    const level = category.class === initialClass ? professionLevel : undefined;
                    const eligibleWeight = category.materials.filter(material => level === undefined || material.minLevel <= level).reduce((total, material) => total + material.chance, 0);
                    return <section key={category.code} className={styles.gatheringCategory}>
                        <header className={styles.monsterHeader}><img src={category.image} alt="" /><div><h3>{l(category.name)}</h3><p>{l(category.class)} · {l(category.professionName)}</p><p>{l(category.description)}</p></div></header>
                        <div className={styles.grid}>{category.materials.map(material => {
                            const available = level === undefined || material.minLevel <= level;
                            const chance = level === undefined ? material.chance : available && eligibleWeight > 0 ? material.chance / eligibleWeight : 0;
                            return <Card as="article" key={material.code} className={styles.card} data-locked={!available}>
                                <img className={styles.materialImage} src={material.image} alt={l(material.name)} loading="lazy" />
                                <h4>{l(material.name)}</h4><p>{l(material.description)}</p>
                                <p><UiText text="Required level" />: {material.minLevel} ({l(professionRankName(material.minLevel))}) · <UiText text="Chance" />: {(chance * 100).toLocaleString(language, { maximumFractionDigits: 2 })}%</p>
                                <p>×1 · <UiText text="Profession materials" /></p>
                                {uses(material.code)}
                                <p><UiText text={material.tradable ? "Tradable after successful extraction." : "Not tradable."} /></p>
                            </Card>;
                        })}</div>
                    </section>;
                })}
            </section>
            <h2><UiText text="Combat profession materials" /></h2>
            <Panel><p>Для профессиональной добычи Гильдия ремесленников должна быть построена до начала матча.</p><p>Профессиональный материал и экипировка выпадают независимо. XP профессии выдаётся только при успешном выпадении материала. Добыча и XP сохраняются после успешного выхода через портал.</p><p>Лимит: {catalog.successLimit} успешных событий добычи на персонажа за матч. Два гоблина — один бросок и две единицы материала.</p></Panel>
            {!monster ? <div className={styles.grid}>{catalog.monsters.map(value => <Card as="article" key={value.code} className={styles.card}>
                <img className={styles.monsterImage} src={value.image} alt={value.name} />
                <h2>{value.name}</h2><p>HP {value.health} · Атака {value.attack} · Защита {value.defense}</p>
                <p>Материалы: {percent(value.professionChance)} · Экипировка: {percent(value.equipmentChance)}</p>
                {value.equipmentChance !== catalog.baseEquipmentChance && <small>Базовый шанс экипировки: {percent(catalog.baseEquipmentChance)}. Действует серверный override.</small>}
                <Button onClick={() => selectMonster(value.code)}>Посмотреть добычу</Button>
            </Card>)}</div> : <>
                <nav className={styles.filters} aria-label="Выбор монстра"><Button onClick={() => selectMonster(null)}>Все монстры</Button>{catalog.monsters.map(value => <Button key={value.code} aria-pressed={value.code === monster.code} onClick={() => selectMonster(value.code)}>{value.name}</Button>)}</nav>
                <Panel className={styles.monsterHeader}><img src={monster.image} alt="" /><div><h2>{monster.name}</h2><p>HP {monster.health} · Атака {monster.attack} · Защита {monster.defense}</p><p>Добыча {percent(monster.professionChance)} · {monster.xp} XP за успех · Экипировка {percent(monster.equipmentChance)}</p></div></Panel>
                <div className={styles.grid}>{monster.materials.filter(value => filter === "all" || value.class === filter).map(value => <Card as="article" key={value.code} className={styles.card}>
                    <img className={styles.materialImage} src={value.image} alt={value.name} loading="lazy" />
                    <h3>{l(value.name)}</h3><p>{className(value.class)} · {value.professionName}</p>
                    <p>×{value.quantity} · Шанс {percent(value.chance)}</p><p>{value.description}</p>
                    {uses(value.code)}
                    <Button onClick={() => setMaterialCode(value.code)}>Открыть материал</Button>
                </Card>)}</div>
                {!monster.materials.some(value => filter === "all" || value.class === filter) && <EmptyState title="Материалов для этого класса нет" />}
            </>}
        </>}
        <dialog ref={detail} className={styles.detail} aria-labelledby="loot-material-title" onCancel={event => { event.preventDefault(); setMaterialCode(null); }} onClick={event => { if (event.target === event.currentTarget) setMaterialCode(null); }}>
            {material && monster && <><Button className={styles.close} onClick={() => setMaterialCode(null)}>Закрыть</Button><img className={styles.largeImage} src={material.image} alt={material.name} /><h2 id="loot-material-title">{l(material.name)}</h2><p>{material.description}</p><dl><dt>Источник</dt><dd>{monster.name}</dd><dt>Класс добытчика</dt><dd>{className(material.class)}</dd><dt>Профессия</dt><dd>{material.professionName}</dd><dt>Шанс за убийство</dt><dd>{percent(material.chance)}</dd><dt>Количество при успехе</dt><dd>×{material.quantity}</dd><dt>Опыт профессии</dt><dd>{monster.xp} XP</dd><dt>{l("Processing and products")}</dt><dd>{uses(material.code)}</dd><dt>Торговля после выхода</dt><dd>{material.tradable ? "Биржа и аукцион" : "Недоступна"}</dd></dl>{material.tradable && <Link href="/market">Открыть рынок</Link>}</>}
        </dialog>
    </div></Wrapper>;
}
