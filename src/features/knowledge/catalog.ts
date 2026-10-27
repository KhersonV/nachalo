import { equipmentImage } from '@/utils/equipmentAssets';
import { editorialArticles, heroChronicles } from './content';
import type { CodexArticle, CodexData } from './types';

export const codexHref = (article: Pick<CodexArticle,'category'|'id'>) => `/drop-guide/${encodeURIComponent(article.category)}/${encodeURIComponent(article.id)}`;
export const itemId = (code: string) => `item-${code}`;
export const recipeId = (code: string) => `recipe-${code}`;
export const materialId = (code: string) => `material-${code}`;
export const monsterId = (id: number) => `monster-${id}`;
export const artifactId = (id: number) => `artifact-${id}`;
export const setId = (code: string) => `set-${code}`;

// A relationship index, not a crafting/loot simulator. Edges use server codes;
// neither prices, eligibility nor drop rolls are computed here.
export function buildCodex(data: CodexData | null, translate: (text: string) => string): CodexArticle[] {
 const articles = editorialArticles.map(article => ({...article,related:[...article.related]}));
 if (!data) return articles;
 for (const hero of data.heroes) {
  const story=heroChronicles[hero.code];
  if (!story) continue;
  articles.push({ id:hero.code,category:'heroes',title:story.title,lead:data.ruleset==='legacy'?'История ордена и базовый профиль активных правил.':story.role,paragraphs:[story.role],lore:story.lore,image:hero.image,related:['class-comparison','reflex','hatred',hero.code==='guardian'?'guardian-aura':hero.code==='berserker'?'fury':hero.code==='ranger'?'critical':'overload',...data.equipment.filter(item=>item.classRestriction===hero.code).map(item=>itemId(item.code))],facts:[hero.code,...(hero.code==='ranger'?['ranger-chain']:[`pve-${hero.code}`])],kind:'hero',entity:hero.code });
 }
 for (const item of data.equipment) {
  const recipe=data.recipes.find(value=>value.code===item.code);
  const set=item.setCode ? [setId(item.setCode)] : [];
  articles.push({id:itemId(item.code),category:'equipment',title:translate(item.name),lead:translate(item.description||item.setName||'Характеристики предмета'),paragraphs:item.description?[translate(item.description)]:[],image:equipmentImage(item.imageUrl,item.code),related:[...(recipe?[recipeId(recipe.code)]:[]),...set,...(item.slot==='ring'||item.slot==='amulet'?['jewelry','reflex']:['slots','bonuses']),...(data.itemSources[item.code]==='monster'?data.monsters.filter(mon=>data.drops.some(drop=>drop.code===mon.name)).map(mon=>monsterId(mon.id)):[])],keywords:[item.code,item.name,item.setName??'',item.rarity,item.slot,item.classRestriction??'',heroChronicles[item.classRestriction??'']?.title??'',translate(({gray:'Common',green:'Uncommon',blue:'Rare',purple:'Epic',orange:'Legendary'} as Record<string,string>)[item.rarity]||item.rarity)],kind:'item',entity:item.code});
 }
 const sets=new Map<string,string>();
 for(const item of data.equipment) if(item.setCode) sets.set(item.setCode,item.setName||item.setCode);
 for(const [code,name] of sets) articles.push({id:setId(code),category:'equipment',title:translate(name),lead:'Предметы существующей серии',paragraphs:['Основные характеристики принадлежат отдельным надетым вещам. У украшений проверяйте отдельно указанный парный бонус.'],related:data.equipment.filter(item=>item.setCode===code).map(item=>itemId(item.code)),kind:'item',entity:`set:${code}`});
 for(const recipe of data.recipes) {
  const item=data.equipment.find(value=>value.code===recipe.code);
  articles.push({id:recipeId(recipe.code),category:'crafting',title:`Рецепт: ${translate(recipe.name)}`,lead:translate(recipe.description),paragraphs:recipe.description?[translate(recipe.description)]:[],image:recipe.image,related:[...(item?[itemId(item.code)]:[materialId(recipe.code)]),...recipe.ingredients.map(ingredient=>materialId(ingredient.code)),'forge-crafting','profession-choice'],keywords:[recipe.code,recipe.name,recipe.ru??'',recipe.de??'',recipe.profession,...recipe.ingredients.map(ingredient=>ingredient.name)],kind:'recipe',entity:recipe.code});
 }
 // Keep the first resource definition, matching ORDER BY id in recipe lookup.
 const resources=new Map(data.resources.map(value=>[value.code,value] as const).reverse());
 for(const recipe of data.recipes) for(const ingredient of recipe.ingredients) if(!resources.has(ingredient.code)) resources.set(ingredient.code,{id:ingredient.resourceId,code:ingredient.code,name:ingredient.name,description:'Материал действующего рецепта.',image:ingredient.image});
 for(const resource of resources.values()) {
  const monsters=data.monsters.filter(mon=>data.drops.some(drop=>drop.code===mon.name&&drop.materials.some(mat=>mat.code===resource.code)));
  const nodes=data.gathering.filter(node=>node.materials.some(mat=>mat.code===resource.code));
  const products=data.recipes.filter(recipe=>recipe.ingredients.some(ingredient=>ingredient.code===resource.code));
  const origins=data.recipes.filter(recipe=>recipe.code===resource.code);
  const relics=data.artifacts.filter(artifact=>(data.dismantle[artifact.code]||[]).some(mat=>mat.code===resource.code));
  const shop=data.shop.filter(item=>item.type===resource.code);
  articles.push({id:materialId(resource.code),category:'crafting',title:translate(resource.name),lead:translate(resource.description||'Ресурс игрового каталога'),paragraphs:resource.description?[translate(resource.description)]:[],image:resource.image,related:[...(data.mapResources.includes(resource.code)?['generation','barrels']:[]),...monsters.map(mon=>monsterId(mon.id)),...nodes.map(node=>`gather-${node.code}`),...origins.map(recipe=>recipeId(recipe.code)),...relics.map(artifact=>artifactId(artifact.id)),...shop.map(item=>`shop-${item.type}`),...products.map(recipe=>recipeId(recipe.code))],keywords:[resource.code,resource.name,...products.map(recipe=>recipe.name)],kind:'material',entity:resource.code});
 }
 for(const mon of data.monsters) {
  const drop=data.drops.find(value=>value.code===mon.name);
  articles.push({id:monsterId(mon.id),category:'bestiary',title:`${translate(drop?.name||mon.name)}${mon.type?` · ${translate(mon.type)}`:''}`,lead:'Обитатель экспедиций: характеристики и известные источники добычи.',paragraphs:['Ближняя атака по живому противнику может вызвать ответный удар. Не приписывайте существу регенерацию или иной навык только по названию выпадающего материала.'],image:mon.image,related:[...(drop?.materials.map(material=>materialId(material.code))||[]),'bestiary-intro','pvp-pve'],keywords:[mon.name,mon.type,drop?.name??'',...(drop?.materials.map(material=>material.name)||[])],lore:[mon.name.toLowerCase().includes('goblin')?'В гоблинских ходах редко бывает тихо: скрежет камня приходит из темноты раньше, чем становится виден сам хозяин тропы.':mon.name.toLowerCase().includes('orc')?'Орочьи следы держатся у старых дорог. Даже разрушенные мосты помнят тяжесть тех, кто проходил здесь после Катастрофы.':mon.name.toLowerCase().includes('troll')?'О тролльих убежищах рассказывают вполголоса. На камне у входа остаются отметины, по которым путники решают искать другую дорогу.':'Путники записывают встречи с этим существом на полях своих карт. Художественная запись не заменяет сведений о его боевых свойствах.'],kind:'monster',entity:String(mon.id)});
 }
 for(const node of data.gathering) articles.push({id:`gather-${node.code}`,category:'crafting',title:translate(node.name),lead:translate(node.description),paragraphs:['Сбор зависит от класса, профессии, уровня и допуска Гильдии на старте. Сервер исключает недоступные награды и нормализует оставшиеся веса.'],image:node.image,related:[...node.materials.map(mat=>materialId(mat.code)),'gathering'],kind:'gathering',entity:node.code});
 for(const artifact of data.artifacts) articles.push({id:artifactId(artifact.id),category:'expeditions',title:translate(artifact.code),lead:translate(artifact.description),paragraphs:[translate(artifact.description),'Показанные свойства относятся к существующему определению. Боевой эффект квестовой реликвии действует только при выполнении условий выбранного артефакта матча.'],image:artifact.image,related:['quest-artifacts','dismantling',...(data.dismantle[artifact.code]||[]).map(mat=>materialId(mat.code))],keywords:[artifact.code],kind:'artifact',entity:String(artifact.id)});
 for(const item of data.shop) articles.push({id:`shop-${item.type}`,category:item.category==='blueprint'?'crafting':'city',title:translate(item.name),lead:translate(item.description),paragraphs:[translate(item.description),'Это определение магазина, а не обещание доступности покупки: личные требования проверяются при действии.'],image:item.image,related:['buying',...(item.category==='blueprint'?['blueprints','map-buildings']:['consumables']),...(resources.has(item.type)?[materialId(item.type)]:[])],keywords:[item.type,item.name],kind:'shop',entity:item.type});
 return articles;
}

export function materialProducts(data:CodexData,code:string) {
 const reachable=new Set([code]);const queue=[code];
 while(queue.length) {
  const current=queue.shift();
  for(const recipe of data.recipes) if(!reachable.has(recipe.code)&&recipe.ingredients.some(ingredient=>ingredient.code===current)) {reachable.add(recipe.code);queue.push(recipe.code);}
 }
 return data.recipes.filter(recipe=>reachable.has(recipe.code)&&recipe.code!==code);
}
