'use client';

import React from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useGameLanguage } from '@/hooks/useGameLanguage';
import { uiText } from '@/i18n/uiText';
import { equipmentRarityName } from '@/utils/equipmentAssets';
import { Button, ErrorState, LoadingState } from '@/components/ui';
import { codexCategories, heroChronicles } from './content';
import { buildCodex, codexHref } from './catalog';
import { useKnowledge } from './KnowledgeProvider';
import type { CodexArticle } from './types';
import CodexEntity, { CodexImage, CodexLinks } from './CodexEntity';
import styles from './KnowledgeCodex.module.css';

const normalize=(text:string)=>text.normalize('NFKC').toLocaleLowerCase('ru').replace(/ё/g,'е');
const kindNames:Record<string,string>={hero:'Класс',item:'Экипировка',recipe:'Рецепт',material:'Материал',monster:'Монстр',artifact:'Артефакт',gathering:'Место сбора',shop:'Магазин',comparison:'Руководство',slots:'Руководство',buildings:'Руководство',professions:'Руководство'};
function pathParts(path:string) {return path.replace(/^\/drop-guide\/?/,'').split('/').filter(Boolean).map(part=>{try{return decodeURIComponent(part)}catch{return part}})}

export default function KnowledgeCodex() {
 const {data,error,loading,refresh}=useKnowledge();const {language}=useGameLanguage();const pathname=usePathname();
 const t=(text:string)=>uiText(language,text);
 const [query,setQuery]=React.useState('');const [typeFilter,setTypeFilter]=React.useState('all');const [limit,setLimit]=React.useState(36);
 const [navOpen,setNavOpen]=React.useState(false);
 React.useEffect(()=>{setQuery('');setTypeFilter('all');setLimit(36);setNavOpen(false)},[pathname]);
 React.useEffect(()=>setLimit(36),[query,typeFilter]);
 const articles=React.useMemo(()=>buildCodex(data,text=>uiText(language,text)),[data,language]);
 const parts=pathParts(pathname);const category=codexCategories.find(value=>value.id===parts[0]);
 const selected=parts.length===2?articles.find(value=>value.category===parts[0]&&value.id===parts[1]):undefined;
 const notFound=parts.length>2||(parts.length>0&&!category)||(parts.length===2&&!selected&&!loading&&!error);
 const searching=Boolean(query.trim());const words=normalize(query.trim()).split(/\s+/).filter(Boolean);
 const visible=React.useMemo(()=>articles.filter(article=>{
  if(!searching&&category&&article.category!==category.id)return false;
  if(typeFilter!=='all'&&(typeFilter==='guide'?Boolean(article.entity):article.kind!==typeFilter))return false;
  const factText=(article.facts||[]).map(id=>data?.facts.find(fact=>fact.id===id)).filter(Boolean).map(fact=>`${fact?.title} ${fact?.text} ${fact?.formula}`).join(' ');
  const content=normalize([article.title,article.lead,...article.paragraphs,...(article.lore||[]),...(article.keywords||[]),factText].join(' '));
  return words.every(word=>content.includes(word));
 }),[articles,category,searching,typeFilter,query,data]);
 const legacyGameplay=data?.ruleset==='legacy'&&selected&&['combat','heroes','stats','equipment'].includes(selected.category);
 const facts=selected?.facts?.flatMap(id=>{const value=data?.facts.find(fact=>fact.id===id);return value?[value]:[]})||[];
 const chapterHref=(id:string)=>`/drop-guide/${encodeURIComponent(id)}`;
 const clear=()=>{setQuery('');setTypeFilter('all')};
 const card=(article:CodexArticle)=>{
  const rarity=article.kind==='item'?data?.equipment.find(item=>item.code===article.entity)?.rarity:article.kind==='recipe'?data?.recipes.find(recipe=>recipe.code===article.entity)?.rarity:undefined;
  return <Link data-rarity={rarity} key={article.id} href={codexHref(article)} className={styles.articleCard} onClick={clear}>
  {article.image&&<CodexImage src={article.image} className={styles.cardImage}/>}
  <span className={styles.cardCopy}><small>{t(kindNames[article.kind||'']||'Статья')} · {t(codexCategories.find(value=>value.id===article.category)?.title||'')}{rarity&&<> · {t(equipmentRarityName(rarity))}</>}</small><strong>{article.title}</strong><span>{article.lead}</span></span><span className={styles.cardArrow} aria-hidden="true">↗</span>
 </Link>;
 };
 return <main className={styles.codex}>
  <header className={styles.masthead}>
   <div className={styles.seal} aria-hidden="true">N<span>✧</span></div><div><p className={styles.eyebrow}>NACHALO · {t('Знания')}</p><h1><Link href="/drop-guide" onClick={clear}>{t('Хроники NACHALO')}</Link></h1><p>{t('История расколотого мира. Искусство возвращаться живым.')}</p></div>
  </header>
  <div className={styles.book}>
   <aside className={styles.sidebar}>
    <button type="button" className={styles.mobileToggle} onClick={()=>setNavOpen(value=>!value)} aria-expanded={navOpen} aria-controls="codex-chapters">{t('Оглавление')} <span aria-hidden="true">{navOpen?'−':'+'}</span></button>
    <nav id="codex-chapters" aria-label={t('Категории Хроник')} className={`${styles.chapterNav} ${navOpen?styles.navOpen:''}`}>
     <Link href="/drop-guide" onClick={clear} className={!parts.length?styles.active:''}>{t('Все главы')}</Link>
     {codexCategories.map(value=><Link key={value.id} href={chapterHref(value.id)} onClick={clear} className={category?.id===value.id?styles.active:''} aria-current={category?.id===value.id?'location':undefined}><span className={styles.numeral}>{value.numeral}</span><span>{t(value.title)}</span></Link>)}
    </nav>
    <p className={styles.sidebarNote}>Истории — на русском языке. Названия предметов и исходные описания используют доступную игровую локализацию.</p>
   </aside>
   <div className={styles.reading}>
    <form role="search" className={styles.search} onSubmit={event=>event.preventDefault()}>
     <label htmlFor="codex-search">{t('Поиск по Хроникам')}</label><div><span aria-hidden="true">⌕</span><input id="codex-search" type="search" value={query} onChange={event=>setQuery(event.target.value)} onKeyDown={event=>{if(event.key==='Escape')clear()}} placeholder={t('Статья, предмет, материал или рецепт…')} autoComplete="off"/>{query&&<button type="button" onClick={clear} aria-label={t('Очистить поиск')}>×</button>}</div>
    </form>
    <nav className={styles.breadcrumbs} aria-label="Хлебные крошки"><Link href="/drop-guide" onClick={clear}>{t('Хроники')}</Link>{category&&<><span aria-hidden="true">/</span><Link href={chapterHref(category.id)} onClick={clear}>{t(category.title)}</Link></>}{selected&&!searching&&<><span aria-hidden="true">/</span><span aria-current="page">{selected.title}</span></>}{searching&&<><span aria-hidden="true">/</span><span aria-current="page">{t('Поиск')}</span></>}</nav>
    {loading&&<LoadingState title="Обновление игровых сведений…"/>}
    {error&&<ErrorState title={error} action={<Button onClick={()=>refresh()}>Повторить загрузку</Button>}/>}
    {data?.unavailable.length? <div className={styles.callout} role="status"><strong>Часть сведений временно недоступна</strong><p>{data.unavailable.join(' · ')}. Истории и доступные разделы можно читать; недостающие проценты не подставляются.</p><Button variant="secondary" onClick={()=>refresh()}>Обновить сведения</Button></div>:null}
    {data&&<p className={styles.ruleset}>Правила новых матчей: <strong>{data.ruleset==='experimental_v34'?'v34':data.ruleset}</strong>. Начатая экспедиция сохраняет собственные правила и характеристики.</p>}
    {notFound?<section className={styles.empty}><h2>Страница не найдена</h2><p>Воспользуйтесь поиском или выберите главу.</p><Link href="/drop-guide" onClick={clear}>Вернуться к оглавлению →</Link></section>:
     selected&&!searching?<article className={styles.article} key={selected.id}>
      <Link href={chapterHref(selected.category)} className={styles.back} onClick={clear}>← {t('К категории')}</Link>
      <header className={styles.articleHeader}>{selected.image&&<CodexImage src={selected.image} alt={selected.title} className={`${styles.articleImage} ${selected.kind==='hero'?styles.heroImage:''}`}/>}<div><p className={styles.eyebrow}>{t(category?.title||'')} · {t(kindNames[selected.kind||'']||'Статья')}</p><h2>{selected.title}</h2><p className={styles.lead}>{selected.lead}</p></div></header>
      {selected.kind==='hero'&&selected.entity&&<p className={styles.subtitle}>{heroChronicles[selected.entity]?.subtitle}</p>}
      {selected.lore&&<section className={styles.lore}><p className={styles.eyebrow}>Из летописей · художественная история</p>{selected.lore.map((paragraph,index)=><p key={index}>{paragraph}</p>)}</section>}
      {legacyGameplay?<p className={styles.callout}>Сервер выбрал legacy для новых матчей. Тактические описания v34 не применяются к нему автоматически; ниже показаны доступные сведения активного профиля и каталог предметов.</p>:selected.paragraphs.map((paragraph,index)=><p key={index}>{paragraph}</p>)}
      {data&&<CodexEntity article={selected} data={data} articles={articles}/>}
      {facts.length>0&&<section className={styles.mechanics}><h2>Игровые правила</h2>{facts.map(fact=><section key={fact.id} className={styles.fact}><h3>{fact.title}</h3><p>{fact.text}</p>{fact.formula&&<pre className={styles.formula}><code>{fact.formula}</code></pre>}</section>)}</section>}
      {selected.facts?.length&&!facts.length&&!loading&&<p className={styles.callout}>Числовые правила этой статьи сейчас не опубликованы активным набором правил. Не используйте значения из другой версии как действующие.</p>}
      {selected.action&&<Link className={styles.action} href={selected.action.href}>{selected.action.label} ↗</Link>}
      <CodexLinks ids={selected.kind==='hero'?selected.related.filter(id=>!id.startsWith('item-')):selected.related} articles={articles}/>
      <footer className={styles.articleFooter}><Link href={chapterHref(selected.category)} onClick={clear}>← {t('К категории')}</Link><Link href="/drop-guide" onClick={clear}>{t('Все главы')} →</Link></footer>
     </article>:
     <>
      {!category&&!searching?<><section className={styles.intro}><p className={styles.eyebrow}>Записи уцелевшей Библиотеки</p><h2>Когда Врата раскололись, память стала дорогой домой.</h2><p>Выберите главу или найдите конкретную вещь. Истории орденов соседствуют с действующими игровыми сведениями — от первого шага по HEX до последнего ингредиента украшения.</p><Link href="/drop-guide/start/first-expedition" className={styles.action}>Я впервые в NACHALO →</Link></section><div className={styles.categoryGrid}>{codexCategories.map(value=><Link key={value.id} href={chapterHref(value.id)} className={styles.categoryCard}><span className={styles.categoryRune} aria-hidden="true">{value.rune}</span><small>Глава {value.numeral}</small><h3>{t(value.title)}</h3><p>{value.subtitle}</p><span className={styles.chapterCount}>{articles.filter(article=>article.category===value.id).length} записей ↗</span></Link>)}</div><h2 className={styles.sectionHeading}>Четыре ордена</h2><div className={styles.heroGrid}>{articles.filter(article=>article.kind==='hero').map(card)}</div></>:
       <section><header className={styles.listHeader}><p className={styles.eyebrow}>{searching?'По всем главам':`Глава ${category?.numeral}`}</p><h2>{searching?`Результаты: «${query.trim()}»`:t(category?.title||'')}</h2>{!searching&&<p>{category?.subtitle}</p>}<p className={styles.muted}>{visible.length} записей</p><label className={styles.typeFilter}>Показать <select value={typeFilter} onChange={event=>setTypeFilter(event.target.value)}><option value="all">Все записи</option><option value="guide">Статьи и руководства</option>{Object.entries(kindNames).filter(([id])=>['hero','item','recipe','material','monster','artifact','shop'].includes(id)).map(([id,name])=><option key={id} value={id}>{name}</option>)}</select></label></header>{visible.length?<><div className={styles.articleGrid}>{visible.slice(0,limit).map(card)}</div>{visible.length>limit&&<Button variant="secondary" onClick={()=>setLimit(value=>value+36)}>Показать ещё ({visible.length-limit})</Button>}</>:<div className={styles.empty}><p>По этому запросу записей не найдено. Попробуйте название материала, предмета или код рецепта.</p>{searching&&<Button onClick={clear}>Сбросить поиск</Button>}</div>}</section>}
     </>}
    <footer className={styles.sources}><span aria-hidden="true">✧</span><p>Числовые сведения — из действующих публичных каталогов игры. Художественная история помечена «Из летописей» и не добавляет боевых свойств.</p>{data?.sources.length?<details><summary>Откуда получены сведения</summary><ul>{data.sources.map(source=><li key={source}>{source}</li>)}</ul></details>:null}</footer>
   </div>
  </div>
 </main>;
}
