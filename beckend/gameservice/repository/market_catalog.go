package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

type MarketStackDefinition struct {
 InventoryKey string
 ID int
 Code string
 Name string
 Category string
 Subcategory string
 Rarity string
 ImageURL string
 Tradable bool
 Stackable bool
 RequiredForgeLevel int
}

// Quest status belongs to a match; extracted copies of other artifact kinds
// are ordinary account stacks and can use the existing stack market.
func MarketArtifactDefinitions(ctx context.Context) ([]MarketStackDefinition, error) {
 rows, err := DB.QueryContext(ctx, `SELECT id,name,COALESCE(image,'') FROM artifacts ORDER BY id`)
 if err != nil { return nil, err }
 defer rows.Close()
 definitions := []MarketStackDefinition{}
 for rows.Next() {
  var id int
  var name, image string
  if err := rows.Scan(&id, &name, &image); err != nil { return nil, err }
  definitions = append(definitions, MarketStackDefinition{InventoryKey: fmt.Sprintf("artifact_%d", id), ID: id, Code: name, Name: name, Category: "artifact", Subcategory: "artifact", Rarity: "gray", ImageURL: image, Tradable: true, Stackable: true})
 }
 return definitions, rows.Err()
}

// Shop catalog maps remain authoritative. Sync keeps the searchable database
// projection current without maintaining a second list of item names.
func SyncMarketStackCatalog(ctx context.Context, definitions []MarketStackDefinition) error {
 tx,err:=DB.BeginTx(ctx,&sql.TxOptions{});if err!=nil{return err};defer tx.Rollback()
 if _,err=tx.ExecContext(ctx,`UPDATE market_stack_catalog SET tradable=false`);err!=nil{return err}
 for _,d:=range definitions {
  if d.InventoryKey==""||d.ID<=0||d.Code==""||d.Name==""||d.Category==""||d.Subcategory=="" {return ErrMarketInvalid}
  if _,err=tx.ExecContext(ctx,`INSERT INTO market_stack_catalog(inventory_key,catalog_id,code,name,category,subcategory,rarity,image_url,tradable,stackable,required_forge_level) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(inventory_key) DO UPDATE SET catalog_id=EXCLUDED.catalog_id,code=EXCLUDED.code,name=EXCLUDED.name,category=EXCLUDED.category,subcategory=EXCLUDED.subcategory,rarity=EXCLUDED.rarity,image_url=EXCLUDED.image_url,tradable=EXCLUDED.tradable,stackable=EXCLUDED.stackable,required_forge_level=EXCLUDED.required_forge_level`,d.InventoryKey,d.ID,d.Code,d.Name,d.Category,d.Subcategory,d.Rarity,d.ImageURL,d.Tradable,d.Stackable,d.RequiredForgeLevel);err!=nil{return err}
 }
 return tx.Commit()
}
func marketStackEligible(ctx context.Context,tx *sql.Tx,key string) error {
 var allowed bool
 err:=tx.QueryRowContext(ctx,`SELECT tradable AND stackable FROM market_stack_catalog WHERE inventory_key=$1`,key).Scan(&allowed)
 if err==sql.ErrNoRows{return ErrMarketInvalid};if err!=nil{return err};if !allowed{return ErrMarketInvalid};return nil
}
func marketStackEntryRestricted(entry map[string]any) bool {
 for _,field:=range []string{"bound","is_bound","soulbound","is_soulbound","locked","is_locked","untradable","reserved"} {
  value,exists:=entry[field];if !exists{continue}
  switch v:=value.(type){case bool:if v{return true};case string:if v!=""&&v!="0"&&!strings.EqualFold(v,"false"){return true};case float64:if v!=0{return true};default:if value!=nil{return true}}
 }
 if value,exists:=entry["tradable"];exists{switch v:=value.(type){case bool:if !v{return true};case string:if strings.EqualFold(v,"false")||v=="0"{return true}}}
 return false
}
func marketStackVariantUnsupported(entry map[string]any)bool{
 for _,field:=range []string{"quality","enhancement","durability","max_durability","item_level","expires_at","bound_to_user_id","bound_to_character_id"}{if _,ok:=entry[field];ok{return true}}
 return false
}
func marketTakeStack(ctx context.Context,tx *sql.Tx,userID int,key string,quantity int)([]byte,error){
 if quantity<1||quantity>MarketMaxQuantity{return nil,ErrMarketInvalid}
 if err:=marketStackEligible(ctx,tx,key);err!=nil{return nil,err}
 var raw []byte;if err:=tx.QueryRowContext(ctx,`SELECT inventory FROM player_profiles WHERE user_id=$1 FOR UPDATE`,userID).Scan(&raw);err!=nil{return nil,err}
 items,err:=parseStorage(raw);if err!=nil{return nil,err};entry:=items[key]
 if marketStackEntryRestricted(entry)||marketStackVariantUnsupported(entry)||storageCount(entry)<quantity{return nil,ErrMarketInventory}
 asset:=map[string]any{};for name,value:=range entry{asset[name]=value};asset["item_count"]=quantity
 left:=storageCount(entry)-quantity;if left==0{delete(items,key)}else{entry["item_count"]=left}
 updated,err:=json.Marshal(items);if err!=nil{return nil,err};data,err:=json.Marshal(asset);if err!=nil{return nil,err}
 _,err=tx.ExecContext(ctx,`UPDATE player_profiles SET inventory=$2::jsonb,updated_at=now() WHERE user_id=$1`,userID,string(updated));return data,err
}
func marketAddStack(ctx context.Context,tx *sql.Tx,userID int,key string,quantity int,data []byte)error{
 var raw []byte;if err:=tx.QueryRowContext(ctx,`SELECT inventory FROM player_profiles WHERE user_id=$1 FOR UPDATE`,userID).Scan(&raw);err!=nil{return err}
 items,err:=parseStorage(raw);if err!=nil{return err};entry:=items[key]
 if entry==nil{if err=json.Unmarshal(data,&entry);err!=nil{return err};entry["item_count"]=0;items[key]=entry}
 if marketStackEntryRestricted(entry)||marketStackVariantUnsupported(entry)||storageCount(entry)+quantity>1_000_000_000{return ErrMarketLimit}
 if strings.HasPrefix(key,"artifact_") {var incoming map[string]any;if err=json.Unmarshal(data,&incoming);err!=nil{return err};for _,field:=range []string{"item_type","item_id","name","item_name","image","description"}{if value,ok:=incoming[field];ok{entry[field]=value}}}
 entry["item_count"]=storageCount(entry)+quantity;updated,err:=json.Marshal(items);if err!=nil{return err}
 _,err=tx.ExecContext(ctx,`UPDATE player_profiles SET inventory=$2::jsonb,updated_at=now() WHERE user_id=$1`,userID,string(updated));return err
}

// MarketCreateOrderItem funds exact equipment-template or canonical stack orders.
// Resource orders continue through the existing resource path.
func MarketCreateOrderItem(ctx context.Context,userID int,key,kind,stackKey string,templateID int64,quantity int,price int64)(id int64,err error){
 if !validMarketAmount(price,quantity){return 0,ErrMarketInvalid}
 if kind=="equipment" {if templateID<=0||quantity!=1||stackKey!=""{return 0,ErrMarketInvalid}} else if kind=="stack" {if stackKey==""||templateID!=0{return 0,ErrMarketInvalid}} else{return 0,ErrMarketInvalid}
 tx,e:=marketTx(ctx);if e!=nil{return 0,e};defer tx.Rollback()
 if id,duplicate,e:=marketIdempotency(ctx,tx,userID,key,"create_order");e!=nil{return 0,e}else if duplicate{return id,tx.Commit()}
 if _,e=tx.ExecContext(ctx,`SELECT pg_advisory_xact_lock(711001,$1)`,userID);e!=nil{return 0,e}
 if kind=="stack"{if e=marketStackEligible(ctx,tx,stackKey);e!=nil{return 0,e}}else{
  var allowed bool
  e=tx.QueryRowContext(ctx,`SELECT tradable AND NOT stackable AND NOT (`+commonEquipmentSQL("")+`) FROM item_templates WHERE id=$1`,templateID).Scan(&allowed)
  if e==sql.ErrNoRows{return 0,ErrMarketInvalid};if e!=nil{return 0,e};if !allowed{return 0,ErrMarketInvalid}
 }
 if e=marketPurchaseEligible(ctx,tx,userID,kind,stackKey);e!=nil{return 0,e}
 var count int;if e=tx.QueryRowContext(ctx,`SELECT count(*) FROM market_orders WHERE buyer_user_id=$1 AND status IN ('open','partially_filled')`,userID).Scan(&count);e!=nil{return 0,e};if count>=MarketOrderLimit{return 0,ErrMarketLimit}
 if _,e=marketProfiles(ctx,tx,userID);e!=nil{return 0,e};total:=price*int64(quantity)
 if e=marketDebit(ctx,tx,userID,total);e!=nil{return 0,e}
 var templateValue,stackValue any;if kind=="equipment"{templateValue=templateID}else{stackValue=stackKey}
 e=tx.QueryRowContext(ctx,`INSERT INTO market_orders(buyer_user_id,item_kind,template_id,stack_key,quantity,price_per_unit,reserved_remaining,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,now()+interval '24 hours') RETURNING id`,userID,kind,templateValue,stackValue,quantity,price,total).Scan(&id);if e!=nil{return 0,e}
 if e=marketResult(ctx,tx,userID,key,id);e!=nil{return 0,e};return id,tx.Commit()
}

func MarketPricePreview(gross int64)(map[string]int64,error){
 if gross<=0||gross>MarketMaxTotal||gross<=marketFee(gross){return nil,ErrMarketInvalid}
 fee:=marketFee(gross)
 return map[string]int64{"gross":gross,"fee":fee,"net":gross-fee},nil
}
