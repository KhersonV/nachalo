package repository

import (
 "context"
 "database/sql"
 "net/url"
 "strconv"
 "time"
)

type MarketSale struct {ID int64 `json:"id"`;Kind string `json:"kind"`;ItemID int64 `json:"itemId"`;StackKey string `json:"stackKey"`;Name string `json:"name"`;ImageURL string `json:"imageUrl"`;Rarity string `json:"rarity"`;Quantity int `json:"quantity"`;UnitPrice float64 `json:"unitPrice"`;Gross int64 `json:"gross"`;Method string `json:"method"`;CreatedAt time.Time `json:"createdAt"`;Durability *int `json:"durability,omitempty"`;MaxDurability *int `json:"maxDurability,omitempty"`}
const marketSalesJoins=` LEFT JOIN market_auctions a ON a.id=t.auction_id LEFT JOIN market_fills f ON f.id=t.fill_id LEFT JOIN market_orders o ON o.id=f.order_id LEFT JOIN item_instances ii ON ii.id=a.item_instance_id LEFT JOIN item_templates it ON it.id=COALESCE(ii.template_id,o.template_id) LEFT JOIN resources r ON r.id=COALESCE(a.resource_id,o.resource_id) LEFT JOIN market_stack_catalog sc ON sc.inventory_key=COALESCE(a.stack_key,o.stack_key) `
const marketSaleKind=`COALESCE(a.item_kind,o.item_kind)`
const marketSaleID=`COALESCE(it.id,r.id,sc.catalog_id,0)`
const marketSaleStack=`COALESCE(a.stack_key,o.stack_key,'')`
const marketSaleQuantity=`COALESCE(f.quantity,a.quantity)`
func MarketCompletedSales(ctx context.Context,page,limit int)([]MarketSale,error){if page<1||page>500||limit<1||limit>50{return nil,ErrMarketInvalid}
 q:=`WITH recent AS (SELECT * FROM market_transactions ORDER BY id DESC LIMIT $1 OFFSET $2) SELECT t.id,`+marketSaleKind+`,`+marketSaleID+`,`+marketSaleStack+`,COALESCE(it.name,r.type,sc.name,'Предмет'),COALESCE(it.image_url,r.image,sc.image_url,''),COALESCE(it.rarity,sc.rarity,'gray'),`+marketSaleQuantity+`,t.gross_price,t.gross_price::float8/`+marketSaleQuantity+`,CASE WHEN t.kind='exchange_fill' THEN 'exchange' WHEN a.highest_bid=t.gross_price THEN 'bid' ELSE 'buyout' END,t.created_at,t.variant_durability,t.variant_max_durability FROM recent t `+marketSalesJoins+` ORDER BY t.id DESC`
 rows,err:=DB.QueryContext(ctx,q,limit,(page-1)*limit);if err!=nil{return nil,err};defer rows.Close();out:=[]MarketSale{};for rows.Next(){var sale MarketSale;var durability,max sql.NullInt64;if err=rows.Scan(&sale.ID,&sale.Kind,&sale.ItemID,&sale.StackKey,&sale.Name,&sale.ImageURL,&sale.Rarity,&sale.Quantity,&sale.Gross,&sale.UnitPrice,&sale.Method,&sale.CreatedAt,&durability,&max);err!=nil{return nil,err};if durability.Valid{v:=int(durability.Int64);sale.Durability=&v};if max.Valid{v:=int(max.Int64);sale.MaxDurability=&v};out=append(out,sale)};return out,rows.Err()}
type MarketPriceDay struct {Day string `json:"day"`;Average float64 `json:"average"`;Units int64 `json:"units"`}
type MarketPriceSummaryData struct {LowestBuyout *int64 `json:"lowestBuyout"`;HighestOrder *int64 `json:"highestOrder"`;Latest *float64 `json:"latest"`;Average7 *float64 `json:"average7"`;Average30 *float64 `json:"average30"`;Units7 int64 `json:"units7"`;Units30 int64 `json:"units30"`;Days []MarketPriceDay `json:"days"`}
func MarketPriceSummary(ctx context.Context,params url.Values)(MarketPriceSummaryData,error){result:=MarketPriceSummaryData{Days:[]MarketPriceDay{}};kind:=params.Get("kind");key:=params.Get("key");if len(key)<1||len(key)>100{return result,ErrMarketInvalid};var id int64;var err error
 if kind=="equipment"||kind=="resource"{id,err=strconv.ParseInt(key,10,64);if err!=nil||id<1{return result,ErrMarketInvalid}}else if kind!="stack"{return result,ErrMarketInvalid}
 durability,maxDurability:=-1,-1
 if params.Get("durability")!=""{durability,err=strconv.Atoi(params.Get("durability"));if err!=nil||durability<0||durability>100000{return result,ErrMarketInvalid}}
 if params.Get("maxDurability")!=""{maxDurability,err=strconv.Atoi(params.Get("maxDurability"));if err!=nil||maxDurability<0||maxDurability>100000{return result,ErrMarketInvalid}}
 if kind!="equipment"&&(durability!=-1||maxDurability!=-1){return result,ErrMarketInvalid}
 match:=`(`+marketSaleKind+`=$1 AND (($1='stack' AND `+marketSaleStack+`=$2) OR ($1<>'stack' AND `+marketSaleID+`=$3)) AND ($1<>'equipment' OR (t.variant_recorded AND COALESCE(t.variant_durability,-1)=$4 AND COALESCE(t.variant_max_durability,-1)=$5)))`
 args:=[]any{kind,key,id,durability,maxDurability}
 q:=`SELECT unit_price FROM (
 SELECT t.gross_price::float8/a.quantity unit_price,t.created_at FROM market_auctions a JOIN market_transactions t ON t.auction_id=a.id LEFT JOIN item_instances ii ON ii.id=a.item_instance_id WHERE a.item_kind=$1 AND (($1='stack' AND a.stack_key=$2) OR ($1='resource' AND a.resource_id=$3) OR ($1='equipment' AND ii.template_id=$3 AND t.variant_recorded AND COALESCE(t.variant_durability,-1)=$4 AND COALESCE(t.variant_max_durability,-1)=$5))
 UNION ALL
 SELECT t.gross_price::float8/f.quantity unit_price,t.created_at FROM market_orders o JOIN market_fills f ON f.order_id=o.id JOIN market_transactions t ON t.fill_id=f.id WHERE o.item_kind=$1 AND (($1='stack' AND o.stack_key=$2) OR ($1='resource' AND o.resource_id=$3) OR ($1='equipment' AND o.template_id=$3 AND t.variant_recorded AND $4=-1 AND $5=-1))
 ) sales ORDER BY created_at DESC LIMIT 1`
 var latest sql.NullFloat64;err=DB.QueryRowContext(ctx,q,args...).Scan(&latest);if err!=nil&&err!=sql.ErrNoRows{return result,err};if latest.Valid{result.Latest=&latest.Float64}
 q=`SELECT (SUM(t.gross_price) FILTER (WHERE t.created_at>=now()-interval '7 days'))::float8 / NULLIF(SUM(`+marketSaleQuantity+`) FILTER (WHERE t.created_at>=now()-interval '7 days'),0),SUM(`+marketSaleQuantity+`) FILTER (WHERE t.created_at>=now()-interval '7 days'),SUM(t.gross_price)::float8/NULLIF(SUM(`+marketSaleQuantity+`),0),SUM(`+marketSaleQuantity+`) FROM market_transactions t `+marketSalesJoins+` WHERE t.created_at>=now()-interval '30 days' AND `+match
 var avg7,avg30 sql.NullFloat64;var units7,units30 sql.NullInt64
 err=DB.QueryRowContext(ctx,q,args...).Scan(&avg7,&units7,&avg30,&units30);if err!=nil{return result,err};if avg7.Valid{result.Average7=&avg7.Float64};if avg30.Valid{result.Average30=&avg30.Float64};result.Units7=units7.Int64;result.Units30=units30.Int64
 q=`SELECT to_char(date_trunc('day',t.created_at),'YYYY-MM-DD'),SUM(t.gross_price)::float8/SUM(`+marketSaleQuantity+`),SUM(`+marketSaleQuantity+`) FROM market_transactions t `+marketSalesJoins+` WHERE t.created_at>=now()-interval '30 days' AND `+match+` GROUP BY 1 ORDER BY 1`
 rows,e:=DB.QueryContext(ctx,q,args...);if e!=nil{return result,e};for rows.Next(){var day MarketPriceDay;if e=rows.Scan(&day.Day,&day.Average,&day.Units);e!=nil{break};result.Days=append(result.Days,day)};if e==nil{e=rows.Err()};rows.Close();if e!=nil{return result,e}
 var low,high sql.NullInt64
 if kind=="stack"{e=DB.QueryRowContext(ctx,`SELECT MIN(buyout_price) FROM market_auctions WHERE item_kind='stack' AND stack_key=$1 AND status='active' AND expires_at>now()`,key).Scan(&low)}else if kind=="resource"{e=DB.QueryRowContext(ctx,`SELECT MIN(buyout_price) FROM market_auctions WHERE item_kind='resource' AND resource_id=$1 AND status='active' AND expires_at>now()`,id).Scan(&low)}else{e=DB.QueryRowContext(ctx,`SELECT MIN(a.buyout_price) FROM market_auctions a JOIN item_instances ii ON ii.id=a.item_instance_id WHERE a.item_kind='equipment' AND ii.template_id=$1 AND COALESCE(ii.durability,-1)=$2 AND COALESCE(ii.max_durability,-1)=$3 AND a.status='active' AND a.expires_at>now()`,id,durability,maxDurability).Scan(&low)};if e!=nil{return result,e};if low.Valid{result.LowestBuyout=&low.Int64}
 if kind=="stack"{e=DB.QueryRowContext(ctx,`SELECT MAX(price_per_unit) FROM market_orders WHERE item_kind='stack' AND stack_key=$1 AND status IN ('open','partially_filled') AND expires_at>now()`,key).Scan(&high)}else if kind=="resource"{e=DB.QueryRowContext(ctx,`SELECT MAX(price_per_unit) FROM market_orders WHERE item_kind='resource' AND resource_id=$1 AND status IN ('open','partially_filled') AND expires_at>now()`,id).Scan(&high)}else if durability==-1&&maxDurability==-1{e=DB.QueryRowContext(ctx,`SELECT MAX(price_per_unit) FROM market_orders WHERE item_kind='equipment' AND template_id=$1 AND status IN ('open','partially_filled') AND expires_at>now()`,id).Scan(&high)};if e!=nil{return result,e};if high.Valid{result.HighestOrder=&high.Int64}
 return result,nil
}
