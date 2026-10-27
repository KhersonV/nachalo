'use client';

import React from 'react';
import Link from 'next/link';
import { uiText } from '@/i18n/uiText';
import { useGameLanguage } from '@/hooks/useGameLanguage';
import { BONUS_FIELDS } from '@/features/equipment/equipmentView';
import { equipmentImage, equipmentRarityName } from '@/utils/equipmentAssets';
import CombatItemStats from '@/components/equipment/CombatItemStats';
import type { EquipmentBonuses, EquipmentItem, EquipmentStats } from '@/types/equipment';
import type { CodexArticle, CodexData } from './types';
import { artifactId, codexHref, itemId, materialId, materialProducts, monsterId, recipeId } from './catalog';
import { heroChronicles } from './content';
import styles from './KnowledgeCodex.module.css';

export function CodexImage({src,alt='',className}:{src?:string;alt?:string;className?:string}) {
 const [failed,setFailed]=React.useState(false);
 React.useEffect(()=>setFailed(false),[src]);
 const safe=src?.startsWith('/')&&!src.startsWith('//')?src:'/ui-icons/backpack.png';
 return <img src={failed?'/ui-icons/backpack.png':safe} alt={alt} className={className} loading="lazy" width={160} height={160} onError={()=>{if(!failed)setFailed(true)}} />;
}
export function BonusTable({bonuses}:{bonuses?:EquipmentBonuses}) {
 const {language}=useGameLanguage();
 const rows=BONUS_FIELDS.filter(field=>(bonuses?.[field.key]??0)!==0);
 return rows.length?<dl className={styles.stats}>{rows.map(field=><div key={field.key}><dt>{uiText(language,field.label)}</dt><dd>{(bonuses?.[field.key]??0)>0?'+':''}{bonuses?.[field.key]}</dd></div>)}</dl>:<p className={styles.muted}>Числовой бонус не указан.</p>;
}
function HeroStats({stats}:{stats:EquipmentStats}) {
 const rows:[string,number|undefined][]=[['HP',stats.maxHealth],['ATK',stats.attack],['DEF',stats.defense],['Reflex',stats.agility],['ОД',stats.maxAP===undefined?undefined:stats.maxAP/2],['ОП',stats.maxMP===undefined?undefined:stats.maxMP/2],['Реакции',stats.maxReactions],['Mobility',stats.mobility],['Обзор',stats.sightRange],['Дальность',stats.attackRange]];
 return <dl className={styles.stats}>{rows.map(([label,value])=><div key={label}><dt>{label}</dt><dd>{value??'—'}</dd></div>)}</dl>;
}
export function CodexLinks({ids,articles,label='Связанные страницы'}:{ids:string[];articles:CodexArticle[];label?:string}) {
 const byId=new Map(articles.map(article=>[article.id,article]));
 const found=Array.from(new Set(ids)).flatMap(id=>byId.get(id)?[byId.get(id)!]:[]);
 if(!found.length) return null;
 return <section className={styles.related}><h3>{label}</h3><div className={styles.linkList}>{found.map(article=><Link href={codexHref(article)} key={article.id}>{article.image&&<CodexImage src={article.image} />}<span>{article.title}</span><span aria-hidden="true">↗</span></Link>)}</div></section>;
}
function DataTable({headers,children}:{headers:string[];children:React.ReactNode}) {
 return <div className={styles.tableWrap}><table><thead><tr>{headers.map(header=><th scope="col" key={header}>{header}</th>)}</tr></thead><tbody>{children}</tbody></table></div>;
}
const effectNames:Record<string,string>={attack:'ATK',defense:'DEF',health:'HP',max_health:'MaxHP',agility:'Reflex',sight_bonus:'Бонус обзора',attack_range:'Дальность',first_step_free:'Бесплатный первый шаг',structure_health:'HP строения',structure_defense:'DEF строения',turret_damage:'Базовый урон турели',structure_blocking:'Блокирование прохода',ap_half:'Внутренние единицы ОД',mp_half:'Внутренние единицы ОП',poison_immunity:'Иммунитет к яду'};
const slots:Record<string,string>={helmet:'Шлем',chest:'Нагрудник',pants:'Штаны',boots:'Сапоги',gloves:'Перчатки',main_hand:'Основная рука',off_hand:'Вторая рука',ring:'Кольцо',amulet:'Амулет'};
export default function CodexEntity({article,data,articles}:{article:CodexArticle;data:CodexData;articles:CodexArticle[]}) {
 const {language}=useGameLanguage();const t=(text:string)=>uiText(language,text);
 const link=(id:string,label?:string)=>{const found=articles.find(value=>value.id===id);return found?<Link href={codexHref(found)}>{label??found.title}</Link>:<span>{label??'Сведения недоступны'}</span>};
 const itemCard=(item:EquipmentItem)=><Link className={styles.itemCard} data-rarity={item.rarity} key={item.code} href={`/drop-guide/equipment/${encodeURIComponent(itemId(item.code))}`}><CodexImage src={equipmentImage(item.imageUrl,item.code)} alt="" /><span><strong>{t(item.name)}</strong><small>{t(equipmentRarityName(item.rarity))} · {slots[item.slot]||item.slot}</small></span></Link>;
 if(article.kind==='hero') {
  const hero=data.heroes.find(value=>value.code===article.entity);const story=hero&&heroChronicles[hero.code];
  if(!hero||!story) return null;
  const progress=data.reflexProgression?.[hero.code];
  if(data.ruleset==='legacy')return <><h2>Базовые характеристики legacy</h2><HeroStats stats={hero.stats}/><h3>Экипировка активных правил</h3><div className={styles.itemGrid}>{data.equipment.filter(item=>item.classRestriction===hero.code).map(itemCard)}</div></>;
  return <><h2>Базовые характеристики</h2><HeroStats stats={hero.stats} />{progress&&<p>Reflex: без вещей <strong>{progress.naked}</strong>; полный Uncommon с кольцом и амулетом <strong>{progress.green}</strong>; Rare с украшениями <strong>{progress.blue}</strong>. Это цели существующих наборов, а не предел характеристики.</p>}<div className={styles.twoColumns}><section><h3>Сильные стороны</h3><p>{story.strengths}</p></section><section><h3>Ограничения</h3><p>{story.weaknesses}</p></section></div><h3>Память ордена</h3><p>{story.enmity}</p><h3>Тактика</h3><ul>{story.tactics.map(text=><li key={text}>{text}</li>)}</ul><CodexLinks ids={hero.code==='mystic'?['overload','magic-penetration','temporary-effects']:hero.code==='guardian'?['counters','guardian-aura','block-dodge']:hero.code==='ranger'?['critical','block-dodge']:['fury','counters']} articles={articles} label="Способности и взаимодействия"/><h3>Доступная экипировка</h3><div className={styles.itemGrid}>{data.equipment.filter(item=>item.classRestriction===hero.code).map(itemCard)}</div></>;
 }
 if(article.kind==='comparison') return <DataTable headers={['Класс','HP','ATK','DEF','Reflex','ОД','ОП','Реакции','Обзор','Дальность']}>{data.heroes.map(hero=><tr key={hero.code}><th scope="row">{link(hero.code,heroChronicles[hero.code]?.title||hero.name)}</th>{[hero.stats.maxHealth,hero.stats.attack,hero.stats.defense,hero.stats.agility,(hero.stats.maxAP??0)/2,(hero.stats.maxMP??0)/2,hero.stats.maxReactions,hero.stats.sightRange,hero.stats.attackRange].map((value,index)=><td key={index}>{value??'—'}</td>)}</tr>)}</DataTable>;
 if(article.kind==='slots') {
  const balance=data.balance;
  return <>{balance&&<><h3>Reflex по слотам</h3><DataTable headers={['Слот','Uncommon','Rare']}>{Object.entries(balance.slot_budgets).map(([slot,budget])=><tr key={slot}><th scope="row">{slots[slot]||slot}</th><td>+{budget.reflex}</td><td>+{balance.rare_slot_reflex?.[slot]??'—'}</td></tr>)}{['ring','amulet'].map(slot=><tr key={slot}><th scope="row">{slots[slot]}</th><td>+{balance.jewelry_reflex?.green?.[slot]??'—'}</td><td>+{balance.jewelry_reflex?.blue?.[slot]??'—'}</td></tr>)}</DataTable><p>Двуручное оружие использует доли обеих рук, щит отдельно не добавляется. Точные характеристики — в карточке вещи.</p>{data.reflexProgression&&<DataTable headers={['Класс','Без вещей','Green + украшения','Blue + украшения']}>{data.heroes.map(hero=><tr key={hero.code}><th scope="row">{link(hero.code,heroChronicles[hero.code]?.title||hero.name)}</th>{['naked','green','blue'].map(tier=><td key={tier}>{data.reflexProgression?.[hero.code]?.[tier]??'—'}</td>)}</tr>)}</DataTable>}</>}<div className={styles.itemGrid}>{data.equipment.filter(item=>article.id!=='jewelry'||item.slot==='ring'||item.slot==='amulet').map(itemCard)}</div></>;
 }
 if(article.kind==='item') {
  if(article.entity?.startsWith('set:')) return <div className={styles.itemGrid}>{data.equipment.filter(item=>item.setCode===article.entity?.slice(4)).map(itemCard)}</div>;
  const item=data.equipment.find(value=>value.code===article.entity);if(!item)return null;
  const recipe=data.recipes.find(value=>value.code===item.code);
  return <><p className={styles.meta} data-rarity={item.rarity}>{t(equipmentRarityName(item.rarity))} · {slots[item.slot]||item.slot} · {item.classRestriction?(heroChronicles[item.classRestriction]?.title||item.classRestriction):'Без ограничения класса'} · Уровень героя: {item.levelRequirement??'не указан'}</p><BonusTable bonuses={item.bonuses}/><CombatItemStats bonuses={item.combatBonuses}/>{item.handedness==='two_hand'&&<p>Двуручное оружие: отдельный щит несовместим.</p>}{item.jewelrySet&&<section className={styles.callout}><h3>{item.jewelrySet.name} · {item.jewelrySet.required} предмета</h3><p>Парный бонус действует при выполнении условия серии, а не на одиночном украшении.</p><BonusTable bonuses={item.jewelrySet.bonus}/></section>}<h3>Получение и изготовление</h3>{recipe?<><p>{link(recipeId(recipe.code),'Открыть текущий рецепт')} · {t(recipe.profession)}, ранг {recipe.minLevel}. Прочность изготовленного экземпляра: {recipe.durability??'не передаётся'}.</p><CodexLinks ids={recipe.ingredients.map(ingredient=>materialId(ingredient.code))} articles={articles} label="Ингредиенты"/></>:<p>{data.itemSources[item.code]==='monster'?'Входит в существующий каталог экипировки из добычи. Смотрите бестиарий и действующие условия броска.':'Рецепт и точный способ получения этой записи не указаны в доступном каталоге. Наличие шаблона не гарантирует предложение в магазине.'}</p>}<p>Прочность и состояние конкретного экземпляра проверяйте в инвентаре. Каталог не раскрывает вещи игроков.</p></>;
 }
 if(article.kind==='recipe') {
  const recipe=data.recipes.find(value=>value.code===article.entity);if(!recipe)return null;
  return <><dl className={styles.stats}>{[['Профессия',t(recipe.profession)],['Требуемый ранг',recipe.minLevel],['Результат за изготовление',recipe.output],['XP за изготовление',recipe.xp],['Категория',t(recipe.category)]].map(([key,value])=><div key={key}><dt>{key}</dt><dd>{value}</dd></div>)}</dl><p>Доступность зависит от выбранного ремесла, уровня, Гильдии и запасов вашего героя.</p><p>{recipe.tradable?'В определении результата торговля разрешена; состояние экземпляра и ограничения рынка проверяются отдельно.':'Торговля результатом не разрешена его определением.'}</p><h3>Ингредиенты одного изготовления</h3><DataTable headers={['Материал','Количество']}>{recipe.ingredients.map(ingredient=><tr key={ingredient.code}><th scope="row"><div className={styles.cellImage}><CodexImage src={ingredient.image}/>{link(materialId(ingredient.code),t(ingredient.name))}</div></th><td>{ingredient.quantity}</td></tr>)}</DataTable>{recipe.bonuses&&<BonusTable bonuses={recipe.bonuses}/>}<CombatItemStats bonuses={recipe.combatBonuses}/>{(recipe.pocketStack??0)>0&&<p>Размер стака в кармане: {recipe.pocketStack}.</p>}{recipe.durability!==undefined&&<p>Прочность результата: {recipe.durability}.</p>}<Link href="/professions" className={styles.action}>Открыть мастерскую ↗</Link></>;
 }
 if(article.kind==='material') {
  const resource=data.resources.find(value=>value.code===article.entity);const code=article.entity||'';
  const drops=data.drops.filter(drop=>drop.materials.some(mat=>mat.code===code));
  const nodes=data.gathering.filter(node=>node.materials.some(mat=>mat.code===code));
  const made=data.recipes.filter(recipe=>recipe.code===code);
  const relics=data.artifacts.filter(artifact=>(data.dismantle[artifact.code]||[]).some(mat=>mat.code===code));
  const sold=data.shop.filter(item=>item.type===code);
  return <><h3>Известные источники</h3>{data.mapResources.includes(code)&&<p className={styles.callout}>Входит в обычный пул ресурсов HEX-экспедиции. Точный шанс конкретной находки не опубликован; он не приравнивается к профессиональному броску.</p>}{!drops.length&&!nodes.length&&!made.length&&!relics.length&&!sold.length&&!data.mapResources.includes(code)&&<p className={styles.callout}>Точный источник и шанс этой записи не определены доступным каталогом. Поиск на рынке не гарантирует наличие.</p>}{drops.map(drop=>{const mat=drop.materials.find(value=>value.code===code)!;return <section key={drop.code} className={styles.callout}><strong>{drop.name}</strong><p>{mat.professionName} · {heroChronicles[mat.class]?.title||mat.class} · ×{mat.quantity}. Шанс профессионального события: {(mat.chance*100).toLocaleString(language)}% при выполнении условий. Это не шанс любого материала для любого класса.</p><CodexLinks ids={data.monsters.filter(mon=>mon.name===drop.code).map(mon=>monsterId(mon.id))} articles={articles} label="Монстры-источники"/></section>})}<CodexLinks ids={[...nodes.map(node=>`gather-${node.code}`),...made.map(recipe=>recipeId(recipe.code)),...relics.map(artifact=>artifactId(artifact.id)),...sold.map(item=>`shop-${item.type}`)]} articles={articles} label="Узлы, изготовление, магазин и разбор"/>{resource?.tradable!==undefined&&<p>{resource.tradable?'Разрешён к торговле при выполнении требований хранения и рынка.':'В определении ресурса торговля запрещена.'}</p>}<CodexLinks ids={materialProducts(data,code).map(recipe=>recipeId(recipe.code))} articles={articles} label="В каких рецептах используется: прямые и последующие изделия"/></>;
 }
 if(article.kind==='monster') {
  const mon=data.monsters.find(value=>String(value.id)===article.entity);if(!mon)return null;
  const drop=data.drops.find(value=>value.code===mon.name);
  return <><dl className={styles.stats}>{[['HP',mon.hp],['ATK',mon.attack],['DEF',mon.defense],['Скорость каталога',mon.speed],['Манёвренность каталога',mon.maneuverability],['Обзор каталога',mon.vision]].map(([key,value])=><div key={key}><dt>{key}</dt><dd>{value}</dd></div>)}</dl><p>Поведение в конкретной HEX-встрече определяется боевым режимом. Каталог не объявляет отдельный навык регенерации или иной специальной способности этой записи.</p>{drop?<><h3>Профессиональная добыча</h3><p>Вероятность события {(drop.professionChance*100).toLocaleString(language)}% · {drop.xp} профессионального XP при успехе и последующем сохранении через выход. Материал определяется профессией/классом добытчика.</p><DataTable headers={['Материал','Профессия / класс','Количество']}>{drop.materials.map(mat=><tr key={mat.code}><th scope="row">{link(materialId(mat.code),t(mat.name))}</th><td>{t(mat.professionName)} · {heroChronicles[mat.class]?.title||mat.class}</td><td>{mat.quantity}</td></tr>)}</DataTable></>:<p className={styles.callout}>Отдельная таблица профессиональных материалов этой записи не опубликована. Другие награды и вероятности не подставляются по аналогии с соседними видами.</p>}{typeof mon.equipmentChance === 'number' && <p>Один независимый бросок Uncommon-экипировки за подтверждённое убийство: {(mon.equipmentChance*100).toLocaleString(language)}%. При успехе — один предмет из разрешённого зелёного пула, без требования профессии или ранга. Это шанс категории, не конкретной вещи.</p>}<CodexLinks ids={data.monsters.filter(value=>value.name===mon.name&&value.id!==mon.id).map(value=>monsterId(value.id))} articles={articles} label="Варианты этого вида"/></>;
 }
 if(article.kind==='gathering') {
  const node=data.gathering.find(value=>value.code===article.entity);if(!node)return null;
  return <><p>{t(node.professionName)} · {heroChronicles[node.class]?.title||node.class}</p><DataTable headers={['Материал','Минимальный ранг','Вес в полной таблице']}>{node.materials.map(mat=><tr key={mat.code}><th scope="row">{link(materialId(mat.code),t(mat.name))}</th><td>{mat.minLevel}</td><td>{(mat.chance*100).toLocaleString(language)}%</td></tr>)}</DataTable><p>Это веса исходного пула. Персональные вероятности после исключения недоступных рангов здесь не вычисляются.</p></>;
 }
 if(article.kind==='artifact') {
  const artifact=data.artifacts.find(value=>String(value.id)===article.entity);if(!artifact)return null;
  const rewards=data.dismantle[artifact.code]||[];
  return <><h3>Параметры определения</h3><dl className={styles.stats}>{Object.entries(artifact.bonus||{}).map(([key,value])=><div key={key}><dt>{effectNames[key]||t(key)}</dt><dd>{value}</dd></div>)}</dl>{rewards.length>0?<><h3>Результат разбора</h3><DataTable headers={['Материал','Количество']}>{rewards.map(reward=><tr key={reward.code}><th scope="row">{link(materialId(reward.code),t(data.resources.find(value=>value.code===reward.code)?.name||reward.code))}</th><td>{reward.quantity}</td></tr>)}</DataTable></>:<p>Разбор для этой записи не указан в действующем каталоге.</p>}</>;
 }
 if(article.kind==='shop') {
  const item=data.shop.find(value=>value.type===article.entity);if(!item)return null;
  return <><p>Цена магазина: <strong>{item.price}</strong> золота. {item.requiresForge?'Требуется Кузница.':''} {item.requiresLibrary?'Для покупки требуется Библиотека.':''}</p>{Object.keys(item.effect||{}).length>0&&<dl className={styles.stats}>{Object.entries(item.effect).map(([key,value])=><div key={key}><dt>{effectNames[key]||t(key)}</dt><dd>{value}</dd></div>)}</dl>}<Link href="/shop" className={styles.action}>Открыть магазин ↗</Link></>;
 }
 if(article.kind==='buildings') return <DataTable headers={['Здание','Дерево','Камень','Железо','Золото']}>{data.buildings.map(building=><tr key={building.code}><th scope="row">{link(building.code==='guild'?'profession-choice':building.code,building.name)}</th>{['wood','stone','iron','gold'].map(key=><td key={key}>{building.costs[key]??0}</td>)}</tr>)}</DataTable>;
 if(article.kind==='professions') return <div className={styles.itemGrid}>{data.professions.map(profession=><section key={profession.code} className={styles.callout}><h3>{t(profession.name)}</h3><p>{t(profession.description)}</p><small>{profession.category==='gathering'?'Добыча по классу':'Переработка на выбор'}</small></section>)}</div>;
 return null;
}
