package repository

import (
 "database/sql"
 _ "embed"
 "encoding/json"
 "fmt"
 "strings"
)

//go:embed jewelry_catalog.json
var jewelryCatalogJSON []byte
//go:embed sql/jewelry_stage_10_4_9.sql
var jewelrySchemaSQL string

type JewelryResource struct {
 Code string `json:"code"`
 Name string `json:"name"`
 RU string `json:"ru"`
 DE string `json:"de"`
 Price int `json:"price"`
}
type JewelryPair struct {
 Name string `json:"name"`
 Bonus EquipmentBonuses `json:"bonus"`
 Required int `json:"required"`
}
type jewelryCatalogData struct {
 Sets []EquipmentCatalogSet `json:"sets"`
 Resources []JewelryResource `json:"resources"`
 Costs map[string]map[string]map[string]int `json:"costs"`
 Profession string `json:"profession"`
 Levels map[string]int `json:"levels"`
 XP map[string]int `json:"xp"`
 Dismantle map[string][]CraftIngredient `json:"dismantle"`
}
var jewelryCatalog = func() jewelryCatalogData {
 var c jewelryCatalogData
 if err := json.Unmarshal(jewelryCatalogJSON, &c); err != nil { panic(err) }
 return c
}()
func JewelryResourceByCode(code string) (JewelryResource, bool) {
 for _, r := range jewelryCatalog.Resources { if r.Code == code { return r,true } }
 return JewelryResource{}, false
}
func JewelryShopTypes() []string {
 out:=[]string{}
 for _, r := range jewelryCatalog.Resources { if r.Price>0 { out=append(out,r.Code) } }
 return out
}
func jewelryItem(code string) (EquipmentCatalogItem, EquipmentCatalogSet, bool) {
 for _, s := range jewelryCatalog.Sets { for _, i := range s.Items { if i.Code==code { return i,s,true } } }
 return EquipmentCatalogItem{},EquipmentCatalogSet{},false
}
func jewelryRecipes() []CraftRecipe {
 out:=[]CraftRecipe{}
 for _, s := range jewelryCatalog.Sets { for _, i := range s.Items {
  ingredients:=[]CraftIngredient{}
  // Stable order for previews and idempotent recipe data.
  for _, material := range jewelryCatalog.Resources { if count:=jewelryCatalog.Costs[s.Rarity][i.Slot][material.Code];count>0 { ingredients=append(ingredients,CraftIngredient{Code:material.Code,Quantity:count}) } }
  ruSlot,deSlot:="Кольцо","Ring"
  if i.Slot=="amulet" { ruSlot,deSlot="Амулет","Amulett" }
  out=append(out,CraftRecipe{Code:i.Code,Name:i.Name,RU:ruSlot+" "+s.Name,DE:s.Name+"-"+deSlot,Profession:jewelryCatalog.Profession,MinLevel:jewelryCatalog.Levels[s.Rarity],XP:jewelryCatalog.XP[s.Rarity],Ingredients:ingredients,Output:1,Category:"jewelry",Slot:i.Slot,HeroClass:s.HeroClass,Rarity:s.Rarity,Image:i.Image,Bonuses:i.Bonuses,JewelrySet:&JewelryPair{Name:s.Name,Bonus:s.PairBonus,Required:2},Tradable:true})
 } }
 return out
}
func seedJewelryTx(tx *sql.Tx) error {
 for _, r := range jewelryCatalog.Resources {
  if _,err:=tx.Exec(`INSERT INTO resources(type,description,effect,image,resource_kind,tradable,market_category,market_subcategory) SELECT $1,$2,'{}','','resource',true,'resource','jewelry_material' WHERE NOT EXISTS(SELECT 1 FROM resources WHERE type=$1)`,r.Code,r.Name);err!=nil{return err}
 }
 for _, s := range jewelryCatalog.Sets {
  var id int64
  if err:=tx.QueryRow(`INSERT INTO item_sets(code,name,class_restriction,rarity,description) VALUES($1,$2,$3,$4,'craft-only jewelry') ON CONFLICT(code) DO UPDATE SET name=EXCLUDED.name RETURNING id`,s.Code,s.Name,s.HeroClass,s.Rarity).Scan(&id);err!=nil{return err}
  for _, i := range s.Items {
   if _,err:=tx.Exec(`INSERT INTO item_templates(code,name,set_id,slot,item_type,handedness,rarity,class_restriction,image_url,attack_bonus,defense_bonus,agility_bonus,max_health_bonus,tradable,npc_price,sell_price) VALUES($1,$2,$3,$4,$4,'none',$5,$6,$11,$7,$8,$9,$10,true,0,0) ON CONFLICT(code) DO UPDATE SET image_url=CASE WHEN EXCLUDED.image_url<>'' THEN EXCLUDED.image_url ELSE item_templates.image_url END,name=EXCLUDED.name,set_id=EXCLUDED.set_id,slot=EXCLUDED.slot,item_type=EXCLUDED.item_type,rarity=EXCLUDED.rarity,class_restriction=EXCLUDED.class_restriction,attack_bonus=EXCLUDED.attack_bonus,defense_bonus=EXCLUDED.defense_bonus,agility_bonus=EXCLUDED.agility_bonus,max_health_bonus=EXCLUDED.max_health_bonus,tradable=true`,i.Code,i.Name,id,i.Slot,s.Rarity,s.HeroClass,i.Bonuses.Attack,i.Bonuses.Defense,i.Bonuses.Agility,i.Bonuses.MaxHealth,i.Image);err!=nil{return err}
  }
  b:=s.PairBonus
  parts:=[]string{}
  if b.Attack!=0 {parts=append(parts,fmt.Sprintf("ATK +%d",b.Attack))}
  if b.Defense!=0 {parts=append(parts,fmt.Sprintf("DEF +%d",b.Defense))}
  if b.MaxHealth!=0 {parts=append(parts,fmt.Sprintf("Max HP +%d",b.MaxHealth))}
  description:=s.Name+" 2/2: "+strings.Join(parts,", ")
  if _,err:=tx.Exec(`INSERT INTO item_set_bonuses(set_id,pieces_required,description,attack_bonus,defense_bonus,max_health_bonus) VALUES($1,2,$2,$3,$4,$5) ON CONFLICT(set_id,pieces_required) DO UPDATE SET description=EXCLUDED.description,attack_bonus=EXCLUDED.attack_bonus,defense_bonus=EXCLUDED.defense_bonus,max_health_bonus=EXCLUDED.max_health_bonus`,id,description,b.Attack,b.Defense,b.MaxHealth);err!=nil{return err}
 }
 return nil
}

// Keep ordinary market transfers untouched; attach the same catalogue pair metadata to cards.
func (item MarketItem) MarshalJSON() ([]byte,error) {
 type MarketItemFields MarketItem
 var pair *JewelryPair
 if _,set,ok:=jewelryItem(item.Code);ok {pair=&JewelryPair{Name:set.Name,Bonus:set.PairBonus,Required:2}}
 return json.Marshal(struct {
  MarketItemFields
  JewelrySet *JewelryPair `json:"jewelrySet,omitempty"`
 }{MarketItemFields:MarketItemFields(item),JewelrySet:pair})
}
