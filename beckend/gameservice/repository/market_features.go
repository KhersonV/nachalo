package repository

import (
 "context"
 "database/sql"
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "strconv"
 "strings"
 "time"
)

var ErrMarketRate = errors.New("market rate limit")
func marketNotice(ctx context.Context,tx *sql.Tx,userID int,key,message,target string,targetID int64)error{
 _,err:=tx.ExecContext(ctx,`INSERT INTO market_notifications(user_id,event_key,message,target_kind,target_id) VALUES($1,$2,$3,$4,$5) ON CONFLICT(event_key) DO NOTHING`,userID,key,message,target,targetID);return err
}
func marketAuctionName(ctx context.Context,tx *sql.Tx,a marketAuctionLock)(string,error){var name string;err:=tx.QueryRowContext(ctx,`SELECT COALESCE(it.name,r.type,sc.name,'Предмет') FROM market_auctions a LEFT JOIN item_instances ii ON ii.id=a.item_instance_id LEFT JOIN item_templates it ON it.id=ii.template_id LEFT JOIN resources r ON r.id=a.resource_id LEFT JOIN market_stack_catalog sc ON sc.inventory_key=a.stack_key WHERE a.id=$1`,a.ID).Scan(&name);return name,err}
func marketOrderName(ctx context.Context,tx *sql.Tx,o marketOrderLock)(string,error){var name string;err:=tx.QueryRowContext(ctx,`SELECT COALESCE(it.name,r.type,sc.name,'Предмет') FROM market_orders o LEFT JOIN item_templates it ON it.id=o.template_id LEFT JOIN resources r ON r.id=o.resource_id LEFT JOIN market_stack_catalog sc ON sc.inventory_key=o.stack_key WHERE o.id=$1`,o.ID).Scan(&name);return name,err}
func MarketNotificationUnreadCount(ctx context.Context,userID int)(int,error){var count int;err:=DB.QueryRowContext(ctx,`SELECT count(*) FROM market_notifications WHERE user_id=$1 AND read_at IS NULL`,userID).Scan(&count);return count,err}
type MarketNotification struct{ID int64 `json:"id"`;Message string `json:"message"`;TargetKind string `json:"targetKind"`;TargetID int64 `json:"targetId"`;Read bool `json:"read"`;CreatedAt time.Time `json:"createdAt"`}
func MarketNotifications(ctx context.Context,userID,page,limit int)([]MarketNotification,int,error){count,err:=MarketNotificationUnreadCount(ctx,userID);if err!=nil{return nil,0,err};rows,err:=DB.QueryContext(ctx,`SELECT id,message,target_kind,target_id,read_at IS NOT NULL,created_at FROM market_notifications WHERE user_id=$1 ORDER BY id DESC LIMIT $2 OFFSET $3`,userID,limit,(page-1)*limit);if err!=nil{return nil,0,err};defer rows.Close();out:=[]MarketNotification{};for rows.Next(){var n MarketNotification;if err=rows.Scan(&n.ID,&n.Message,&n.TargetKind,&n.TargetID,&n.Read,&n.CreatedAt);err!=nil{return nil,0,err};out=append(out,n)};return out,count,rows.Err()}
func MarketReadNotification(ctx context.Context,userID int,id int64)error{result,err:=DB.ExecContext(ctx,`UPDATE market_notifications SET read_at=COALESCE(read_at,now()) WHERE id=$1 AND user_id=$2`,id,userID);if err!=nil{return err};n,_:=result.RowsAffected();if n==0{return ErrMarketUnavailable};return nil}
func marketRateLimit(name string,fallback int)int{value,err:=strconv.Atoi(os.Getenv("MARKET_RATE_"+name));if err!=nil||value<1||value>100000{return fallback};return value}
// A database advisory lock makes each account/category window consistent across processes.
func MarketRateCheck(ctx context.Context,userID int,category string)error{
 var limit int;var window time.Duration
 switch category{case "api":limit=marketRateLimit("API_PER_MINUTE",240);window=time.Minute
 case "create":limit=marketRateLimit("CREATE_PER_10_MINUTES",12);window=10*time.Minute
 case "cancel":limit=marketRateLimit("CANCEL_PER_10_MINUTES",20);window=10*time.Minute
 case "bid":limit=marketRateLimit("BID_PER_10_MINUTES",60);window=10*time.Minute
 default:return ErrMarketInvalid}
 tx,err:=marketTx(ctx);if err!=nil{return err};defer tx.Rollback()
 code:=map[string]int{"api":1,"create":2,"cancel":3,"bid":4}[category]
 if _,err=tx.ExecContext(ctx,`SELECT pg_advisory_xact_lock($1,$2)`,712000+code,userID);err!=nil{return err}
 var count int
 if err=tx.QueryRowContext(ctx,`SELECT count(*) FROM market_rate_events WHERE user_id=$1 AND category=$2 AND created_at>now()-$3::interval`,userID,category,fmt.Sprintf("%d seconds",int(window.Seconds()))).Scan(&count);err!=nil{return err}
 if count>=limit{return ErrMarketRate}
 if _,err=tx.ExecContext(ctx,`INSERT INTO market_rate_events(user_id,category) VALUES($1,$2)`,userID,category);err!=nil{return err}
 return tx.Commit()
}
func MarketAudit(ctx context.Context,userID int,event,kind string,subjectID int64,details any){raw,err:=json.Marshal(details);if err!=nil{return};_,_=DB.ExecContext(ctx,`INSERT INTO market_audit(actor_user_id,event,subject_kind,subject_id,details) VALUES($1,$2,$3,$4,$5::jsonb)`,userID,event,kind,subjectID,string(raw))}
func marketAuditTx(ctx context.Context,tx *sql.Tx,userID int,event,kind string,subjectID int64,details any)error{if values,ok:=details.(map[string]any);ok&&(event=="auction_sale"||event=="exchange_fill"){marketAddPriceDeviation(ctx,values)};raw,err:=json.Marshal(details);if err!=nil{return err};_,err=tx.ExecContext(ctx,`INSERT INTO market_audit(actor_user_id,event,subject_kind,subject_id,details) VALUES($1,$2,$3,$4,$5::jsonb)`,userID,event,kind,subjectID,string(raw));return err}

// Compare executed unit prices with actual settled sales; log the ratio only.
func marketAddPriceDeviation(ctx context.Context,values map[string]any){
 kind,_:=values["itemKind"].(string);var key string;var id int64
 if kind=="stack"{key,_=values["stackKey"].(string)}else if kind=="resource"{if v,ok:=values["resourceId"].(int);ok{id=int64(v)}}else if kind=="equipment"{id,_=values["templateId"].(int64)}
 if kind==""||(kind=="stack"&&key=="")||(kind!="stack"&&id<=0){return}
 unit:=0.0;switch v:=values["unitPrice"].(type){case float64:unit=v;case int64:unit=float64(v)};if unit<=0{return}
 durability,maxDurability:=int64(-1),int64(-1)
 if values["hasDurability"]==true{durability,_=values["variantDurability"].(int64)}
 if values["hasMaxDurability"]==true{maxDurability,_=values["variantMaxDurability"].(int64)}
 q:=`SELECT SUM(t.gross_price)::float8/NULLIF(SUM(`+marketSaleQuantity+`),0) FROM market_transactions t `+marketSalesJoins+` WHERE t.created_at>=now()-interval '30 days' AND `+marketSaleKind+`=$1 AND (($1='stack' AND `+marketSaleStack+`=$2) OR ($1<>'stack' AND `+marketSaleID+`=$3)) AND ($1<>'equipment' OR (t.variant_recorded AND COALESCE(t.variant_durability,-1)=$4 AND COALESCE(t.variant_max_durability,-1)=$5))`
 var average sql.NullFloat64;if err:=DB.QueryRowContext(ctx,q,kind,key,id,durability,maxDurability).Scan(&average);err!=nil||!average.Valid||average.Float64<=0{return}
 ratio:=unit/average.Float64;values["average30BeforeTrade"]=average.Float64;values["priceRatio"]=ratio;if ratio>=5||ratio<=0.2{values["extremePriceDeviation"]=true}
}
// Favorites contain canonical item identifiers, never ownership records.
type MarketFavorite struct{Kind string `json:"kind"`;Key string `json:"key"`;Name string `json:"name"`;ImageURL string `json:"imageUrl"`;Rarity string `json:"rarity"`;Auctions int `json:"auctions"`;Orders int `json:"orders"`}
func MarketFavorites(ctx context.Context,userID int)([]MarketFavorite,error){rows,err:=DB.QueryContext(ctx,`SELECT f.item_kind,f.item_key,COALESCE(it.name,r.type,sc.name,'Предмет'),COALESCE(it.image_url,r.image,sc.image_url,''),COALESCE(it.rarity,sc.rarity,'gray'),
 (SELECT count(*) FROM market_auctions a LEFT JOIN item_instances ii ON ii.id=a.item_instance_id WHERE a.status='active' AND a.expires_at>now() AND a.item_kind=f.item_kind AND ((f.item_kind='equipment' AND ii.template_id=it.id ) OR (f.item_kind='resource' AND a.resource_id=r.id) OR (f.item_kind='stack' AND a.stack_key=f.item_key))),
 (SELECT count(*) FROM market_orders o WHERE o.status IN ('open','partially_filled') AND o.expires_at>now() AND o.item_kind=f.item_kind AND ((f.item_kind='equipment' AND o.template_id=it.id) OR (f.item_kind='resource' AND o.resource_id=r.id) OR (f.item_kind='stack' AND o.stack_key=f.item_key)))
 FROM market_favorites f LEFT JOIN item_templates it ON f.item_kind='equipment' AND f.item_key=it.id::text LEFT JOIN resources r ON f.item_kind='resource' AND f.item_key=r.id::text LEFT JOIN market_stack_catalog sc ON f.item_kind='stack' AND f.item_key=sc.inventory_key WHERE f.user_id=$1 ORDER BY f.created_at DESC LIMIT 100`,userID);if err!=nil{return nil,err};defer rows.Close();out:=[]MarketFavorite{};for rows.Next(){var f MarketFavorite;if err=rows.Scan(&f.Kind,&f.Key,&f.Name,&f.ImageURL,&f.Rarity,&f.Auctions,&f.Orders);err!=nil{return nil,err};out=append(out,f)};return out,rows.Err()}
func MarketFavoriteChange(ctx context.Context,userID int,kind,key string,add bool)error{
 if len(key)<1||len(key)>100{return ErrMarketInvalid};if kind!="equipment"&&kind!="resource"&&kind!="stack"{return ErrMarketInvalid};if !add{_,err:=DB.ExecContext(ctx,`DELETE FROM market_favorites WHERE user_id=$1 AND item_kind=$2 AND item_key=$3`,userID,kind,key);return err};var exists bool
 switch kind{case "equipment":id,e:=strconv.ParseInt(key,10,64);if e!=nil||id<1{return ErrMarketInvalid};if e=DB.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM item_templates WHERE id=$1 AND tradable)`,id).Scan(&exists);e!=nil{return e}
 case "resource":id,e:=strconv.ParseInt(key,10,64);if e!=nil||id<1{return ErrMarketInvalid};if e=DB.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM resources WHERE id=$1 AND tradable)`,id).Scan(&exists);e!=nil{return e}
 case "stack":if e:=DB.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM market_stack_catalog WHERE inventory_key=$1 AND tradable)`,key).Scan(&exists);e!=nil{return e}
 default:return ErrMarketInvalid}
 if !exists{return ErrMarketInvalid}
 tx,err:=marketTx(ctx);if err!=nil{return err};defer tx.Rollback();if _,err=tx.ExecContext(ctx,`SELECT pg_advisory_xact_lock(713000,$1)`,userID);err!=nil{return err};var count int;if err=tx.QueryRowContext(ctx,`SELECT count(*) FROM market_favorites WHERE user_id=$1`,userID).Scan(&count);err!=nil{return err};if count>=100{return ErrMarketLimit};_,err=tx.ExecContext(ctx,`INSERT INTO market_favorites(user_id,item_kind,item_key) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`,userID,kind,key);if err!=nil{return err};return tx.Commit()
}
// Validated filters are stored as account preferences, not executable SQL.
type MarketSavedSearch struct{ID int64 `json:"id"`;Name string `json:"name"`;Filters json.RawMessage `json:"filters"`}
func MarketSavedSearches(ctx context.Context,userID int)([]MarketSavedSearch,error){rows,err:=DB.QueryContext(ctx,`SELECT id,name,filters FROM market_saved_searches WHERE user_id=$1 ORDER BY id DESC LIMIT 20`,userID);if err!=nil{return nil,err};defer rows.Close();out:=[]MarketSavedSearch{};for rows.Next(){var s MarketSavedSearch;var raw []byte;if err=rows.Scan(&s.ID,&s.Name,&raw);err!=nil{return nil,err};s.Filters=json.RawMessage(raw);out=append(out,s)};return out,rows.Err()}
func marketValidSavedFilters(raw json.RawMessage)error{
 if len(raw)==0||len(raw)>2048{return ErrMarketInvalid}
 var filters map[string]any;if err:=json.Unmarshal(raw,&filters);err!=nil{return ErrMarketInvalid}
 allowed:=map[string]int{"mode":20,"search":100,"category":40,"subcategory":80,"rarity":20,"minLevel":6,"maxLevel":6,"class":20,"minPrice":12,"maxPrice":12,"sort":30}
 if len(filters)>len(allowed){return ErrMarketInvalid}
 for key,value:=range filters{max,ok:=allowed[key];if !ok{return ErrMarketInvalid};text,ok:=value.(string);if !ok||len(text)>max{return ErrMarketInvalid}
  switch key{case "mode":if text!="auction"&&text!="exchange"{return ErrMarketInvalid}
  case "rarity":if text!=""&&text!="gray"&&text!="green"&&text!="blue"&&text!="purple"&&text!="orange"{return ErrMarketInvalid}
  case "class":if text!=""&&text!="guardian"&&text!="berserker"&&text!="ranger"&&text!="mystic"{return ErrMarketInvalid}
  case "sort":if text!=""&&text!="newest"&&text!="lowest"&&text!="highest"&&text!="expiring"&&text!="highest_bid"&&text!="buyout"&&text!="remaining"&&text!="level"{return ErrMarketInvalid}
  case "minLevel","maxLevel","minPrice","maxPrice":if text!=""{n,err:=strconv.ParseInt(text,10,64);limit:=int64(10000);if key=="minPrice"||key=="maxPrice"{limit=MarketMaxPrice};if err!=nil||n<0||n>limit{return ErrMarketInvalid}}}
 }
 return nil
}
func MarketSaveSearch(ctx context.Context,userID int,name string,filters json.RawMessage)(int64,error){name=strings.TrimSpace(name);if len([]rune(name))<1||len([]rune(name))>40{return 0,ErrMarketInvalid};if err:=marketValidSavedFilters(filters);err!=nil{return 0,err};tx,err:=marketTx(ctx);if err!=nil{return 0,err};defer tx.Rollback();if _,err=tx.ExecContext(ctx,`SELECT pg_advisory_xact_lock(713001,$1)`,userID);err!=nil{return 0,err};var count int;if err=tx.QueryRowContext(ctx,`SELECT count(*) FROM market_saved_searches WHERE user_id=$1`,userID).Scan(&count);err!=nil{return 0,err};if count>=20{return 0,ErrMarketLimit};var id int64;err=tx.QueryRowContext(ctx,`INSERT INTO market_saved_searches(user_id,name,filters) VALUES($1,$2,$3::jsonb) ON CONFLICT(user_id,name) DO NOTHING RETURNING id`,userID,name,string(filters)).Scan(&id);if err==sql.ErrNoRows{return 0,ErrMarketInvalid};if err!=nil{return 0,err};return id,tx.Commit()}
func MarketRenameSearch(ctx context.Context,userID int,id int64,name string)error{name=strings.TrimSpace(name);if len([]rune(name))<1||len([]rune(name))>40{return ErrMarketInvalid};result,err:=DB.ExecContext(ctx,`UPDATE market_saved_searches SET name=$3 WHERE id=$1 AND user_id=$2 AND NOT EXISTS(SELECT 1 FROM market_saved_searches other WHERE other.user_id=$2 AND other.name=$3 AND other.id<>$1)`,id,userID,name);if err!=nil{return err};n,_:=result.RowsAffected();if n==0{return ErrMarketUnavailable};return nil}
func MarketDeleteSearch(ctx context.Context,userID int,id int64)error{result,err:=DB.ExecContext(ctx,`DELETE FROM market_saved_searches WHERE id=$1 AND user_id=$2`,id,userID);if err!=nil{return err};n,_:=result.RowsAffected();if n==0{return ErrMarketUnavailable};return nil}
