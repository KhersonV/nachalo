package repository

import (
	"context"
	"database/sql"
    "encoding/json"
	"strconv"
	"strings"
	"time"
)

type MarketFilter struct {
	Search, Category, Subcategory, Rarity, Class, Status, Sort, Party, ItemKind, ItemKey string
	MinLevel, MaxLevel                                                int
	MinPrice, MaxPrice, TargetID                                      int64
	Page, Limit                                                       int
}

func (f *MarketFilter) Validate() error {
	if f.TargetID<0{return ErrMarketInvalid};if f.Page < 1 || f.Page > 100000 || f.Limit < 1 || f.Limit > 50 || f.MinLevel < 0 || f.MaxLevel < 0 || f.MinLevel > 10000 || f.MaxLevel > 10000 || f.MinPrice < 0 || f.MaxPrice < 0 || f.MinPrice > MarketMaxPrice || f.MaxPrice > MarketMaxPrice {
		return ErrMarketInvalid
	}
	if f.MaxLevel > 0 && f.MaxLevel < f.MinLevel || f.MaxPrice > 0 && f.MaxPrice < f.MinPrice {
		return ErrMarketInvalid
	}
	if len(f.ItemKey)>100 || (f.ItemKind!="" && f.ItemKind!="equipment" && f.ItemKind!="resource" && f.ItemKind!="stack") || (f.ItemKind=="")!=(f.ItemKey=="") {return ErrMarketInvalid}
	if len(f.Search) > 100 || len(f.Party) > 80 {
		return ErrMarketInvalid
	}
    if len(f.Category)>40||len(f.Subcategory)>80{return ErrMarketInvalid}
	if f.Rarity != "" && f.Rarity != "gray" && f.Rarity != "green" && f.Rarity != "blue" && f.Rarity != "purple" && f.Rarity != "orange" {
		return ErrMarketInvalid
	}
	if f.Class != "" && f.Class != "guardian" && f.Class != "berserker" && f.Class != "ranger" && f.Class != "mystic" {
		return ErrMarketInvalid
	}
	if f.Sort != "" && f.Sort != "lowest" && f.Sort != "highest" && f.Sort != "newest" && f.Sort != "expiring" && f.Sort != "highest_bid" && f.Sort != "level" && f.Sort != "buyout" && f.Sort != "remaining" {
		return ErrMarketInvalid
	}
	return nil
}

type MarketItem struct {InstanceID string `json:"instanceId,omitempty"`;Quantity int `json:"quantity,omitempty"`;Durability *int `json:"durability,omitempty"`;MaxDurability *int `json:"maxDurability,omitempty"`;RequiredForgeLevel int `json:"requiredForgeLevel,omitempty"`;Attributes map[string]int `json:"attributes,omitempty"`;
	StackKey string `json:"stackKey,omitempty"`
 Stackable bool `json:"stackable"`
 ExchangeEligible bool `json:"exchangeEligible"`
 Kind        string `json:"kind"`
	ID          int64  `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Subcategory string `json:"subcategory"`
	Rarity      string `json:"rarity"`
	ImageURL    string `json:"imageUrl"`
	Level       int    `json:"levelRequirement"`
	Class       string `json:"classRestriction"`
	Tradable    bool   `json:"tradable"`
}
type MarketAuction struct {
	ID          int64      `json:"id"`
	SellerID    int        `json:"sellerId"`
	Seller      string     `json:"seller"`
	Item        MarketItem `json:"item"`
	InstanceID  string     `json:"instanceId,omitempty"`
	Quantity    int        `json:"quantity"`
	StartPrice  int64      `json:"startPrice"`
	BuyoutPrice *int64     `json:"buyoutPrice"`
	HighestBid  *int64     `json:"highestBid"`
	BidCount    int        `json:"bidCount"`
	Status      string     `json:"status"`
	ExpiresAt   time.Time  `json:"expiresAt"`
	CreatedAt   time.Time  `json:"createdAt"`
}
type MarketOrder struct {
	ID                int64      `json:"id"`
	BuyerID           int        `json:"buyerId"`
	Buyer             string     `json:"buyer"`
	Item              MarketItem `json:"item"`
	Quantity          int        `json:"quantity"`
	FilledQuantity    int        `json:"filledQuantity"`
	RemainingQuantity int        `json:"remainingQuantity"`
	PricePerUnit      int64      `json:"pricePerUnit"`
	ReservedRemaining int64      `json:"reservedRemaining"`
	Status            string     `json:"status"`
	ExpiresAt         time.Time  `json:"expiresAt"`
	CreatedAt         time.Time  `json:"createdAt"`
}
type MarketDelivery struct {
	ID         int64      `json:"id"`
	Item       MarketItem `json:"item"`
	InstanceID string     `json:"instanceId,omitempty"`
	Quantity   int        `json:"quantity"`
	CreatedAt  time.Time  `json:"createdAt"`
}
type MarketHistory struct {
	ID        int64     `json:"id"`
	Kind      string    `json:"kind"`
	BuyerID   int       `json:"buyerId"`
	SellerID  int       `json:"sellerId"`
	Gross     int64     `json:"gross"`
	Fee       int64     `json:"fee"`
	CreatedAt time.Time `json:"createdAt"`
}

const marketResourceCategorySQL = "COALESCE(NULLIF(r.market_category,''),CASE WHEN r.type IN ('food','water','potion','elixir') THEN 'consumable' ELSE 'resource' END)"
func marketResourceCategory(itemType string)string{switch itemType{case "food","water","potion","elixir":return "consumable";default:return "resource"}}
const marketEquipmentCategorySQL = "CASE WHEN it.item_type IN ('sword','axe','bow','staff','shield') THEN 'weapon' WHEN it.item_type IN ('helmet','chest','gloves','pants','boots') THEN 'armor' WHEN it.item_type IN ('ring','amulet') THEN 'accessory' ELSE 'special' END"
func marketCategory(sub,kind string)string{if kind=="resource"{return "resource"};switch sub{case "sword","axe","bow","staff","shield":return "weapon";case "helmet","chest","gloves","pants","boots":return "armor";case "ring","amulet":return "accessory";default:return "special"}}
func marketSQLArg(args *[]any, v any) string {
	*args = append(*args, v)
	return "$" + strconv.Itoa(len(*args))
}
func marketOrderBy(sortKey string, auction bool) string {
	switch sortKey {
	case "lowest":
		if auction {
			return "COALESCE(a.highest_bid,a.start_price) ASC,a.id DESC"
		}
		return "o.price_per_unit ASC,o.id DESC"
	case "highest":
		if auction {
			return "COALESCE(a.highest_bid,a.start_price) DESC,a.id DESC"
		}
		return "o.price_per_unit DESC,o.id DESC"
    case "buyout": if auction {return "a.buyout_price ASC NULLS LAST,a.id DESC"};return "o.price_per_unit DESC,o.id DESC"
    case "remaining": if auction{return "a.created_at DESC,a.id DESC"};return "(o.quantity-o.filled_quantity) DESC,o.id DESC"
	case "expiring":
		if auction {
			return "a.expires_at ASC,a.id DESC"
		}
		return "o.expires_at ASC,o.id DESC"
	case "highest_bid":
		if auction {
			return "COALESCE(a.highest_bid,0) DESC,a.id DESC"
		}
		return "o.price_per_unit DESC,o.id DESC"
	case "level":
		if auction {
			return "COALESCE(it.level_requirement,1) DESC,a.id DESC"
		}
		return "COALESCE(it.level_requirement,1) DESC,o.id DESC"
	default:
		if auction {
			return "a.created_at DESC,a.id DESC"
		}
		return "o.created_at DESC,o.id DESC"
	}
}

const marketAttributesSQL = "jsonb_build_object('attack',it.attack_bonus,'defense',it.defense_bonus,'mobility',it.mobility_bonus,'agility',it.agility_bonus,'health',it.max_health_bonus,'energy',it.max_energy_bonus,'sight',it.sight_range_bonus,'range',it.attack_range_bonus)"
func marketItemAttributes(raw []byte) map[string]int {var values map[string]int;if len(raw)>0{_ = json.Unmarshal(raw,&values)};return values}
func marketScanItem(scan func(...any)error)(item MarketItem,err error){
 var class sql.NullString;var attributes []byte
 err=scan(&item.Kind,&item.ID,&item.StackKey,&item.Code,&item.Name,&item.Category,&item.Subcategory,&item.Rarity,&item.ImageURL,&item.Level,&class,&item.Stackable,&item.RequiredForgeLevel,&attributes)
 if err==nil{item.Class=class.String;item.Attributes=marketItemAttributes(attributes);item.Tradable=true;item.ExchangeEligible=(item.Kind=="equipment"&&!item.Stackable)||(item.Kind!="equipment"&&item.Stackable)};return
}
func MarketCatalog(ctx context.Context,f MarketFilter)([]MarketItem,error){
 if err:=f.Validate();err!=nil{return nil,err};args:=[]any{};where:=[]string{"true"}
 if f.Search!=""{where=append(where,"lower(name) LIKE lower("+marketSQLArg(&args,"%"+f.Search+"%")+")")}
 if f.Category!=""{where=append(where,"category="+marketSQLArg(&args,f.Category))}
 if f.Subcategory!=""{where=append(where,"subcategory="+marketSQLArg(&args,f.Subcategory))}
 if f.Rarity!=""{where=append(where,"rarity="+marketSQLArg(&args,f.Rarity))}
 if f.Class!=""{where=append(where,"(class_restriction IS NULL OR class_restriction="+marketSQLArg(&args,f.Class)+")")}
 if f.MinLevel>0{where=append(where,"level_requirement >= "+marketSQLArg(&args,f.MinLevel))}
 if f.MaxLevel>0{where=append(where,"level_requirement <= "+marketSQLArg(&args,f.MaxLevel))}
 q:=`SELECT kind,id,stack_key,code,name,category,subcategory,rarity,image_url,level_requirement,class_restriction,stackable,required_forge_level,attributes FROM (
 SELECT 'equipment'::text kind,id,''::text stack_key,code,name,`+marketEquipmentCategorySQL+` category,item_type subcategory,rarity,image_url,level_requirement,class_restriction,stackable,0 required_forge_level,`+marketAttributesSQL+` attributes FROM item_templates it WHERE tradable=true
 UNION ALL SELECT 'resource',id::bigint,'','resource_'||id,type,`+marketResourceCategorySQL+`,COALESCE(NULLIF(r.market_subcategory,''),r.type),'gray',COALESCE(image,''),1,NULL::text,true,0,'{}'::jsonb FROM resources r WHERE r.tradable
 UNION ALL SELECT 'stack',catalog_id::bigint,inventory_key,code,name,category,subcategory,rarity,image_url,1,NULL::text,stackable,required_forge_level,'{}'::jsonb FROM market_stack_catalog WHERE tradable=true
 ) catalog WHERE `+strings.Join(where," AND ")+` ORDER BY name,kind,id,stack_key LIMIT `+marketSQLArg(&args,f.Limit)+` OFFSET `+marketSQLArg(&args,(f.Page-1)*f.Limit)
 rows,e:=DB.QueryContext(ctx,q,args...);if e!=nil{return nil,e};defer rows.Close();out:=[]MarketItem{}
 for rows.Next(){item,e:=marketScanItem(rows.Scan);if e!=nil{return nil,e};out=append(out,item)};return out,rows.Err()
}
func MarketInventory(ctx context.Context, userID int) ([]MarketItem, error) {
	items := []MarketItem{}
	rows, e := DB.QueryContext(ctx, `SELECT ii.id::text,it.id,it.code,it.name,it.item_type,it.rarity,it.image_url,it.level_requirement,it.class_restriction,`+marketAttributesSQL+`,(ii.durability IS NULL AND ii.max_durability IS NULL),ii.durability,ii.max_durability FROM item_instances ii JOIN item_templates it ON it.id=ii.template_id WHERE ii.owner_user_id=$1 AND ii.status='inventory' AND ii.current_character_id IS NULL AND ii.match_instance_id IS NULL AND ii.bound_to_user_id IS NULL AND ii.bound_to_character_id IS NULL AND NOT ii.is_soulbound AND NOT ii.is_locked AND it.tradable AND NOT EXISTS(SELECT 1 FROM character_equipment ce WHERE ce.item_instance_id=ii.id) ORDER BY it.name`, userID)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var instance, code, name, sub, rarity, image string
		var id int64
		var level int
		var class sql.NullString
        var attributes []byte
        var full bool
        var durability,maxDurability sql.NullInt64
		if e = rows.Scan(&instance, &id, &code, &name, &sub, &rarity, &image, &level, &class, &attributes, &full, &durability, &maxDurability); e != nil {
			break
		}
		items = append(items, MarketItem{Kind: "equipment", ID: id, Code: code, Name: name, Category: marketCategory(sub,"equipment"), Subcategory: sub, Rarity: rarity, ImageURL: image, Level: level, Class: class.String, Tradable: true})
		items[len(items)-1].Attributes = marketItemAttributes(attributes)
        items[len(items)-1].InstanceID = instance
        items[len(items)-1].Quantity = 1
        items[len(items)-1].ExchangeEligible = full
        if durability.Valid { value:=int(durability.Int64);items[len(items)-1].Durability=&value }
        if maxDurability.Valid { value:=int(maxDurability.Int64);items[len(items)-1].MaxDurability=&value }
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return nil, e
	}
	var raw []byte
	if e = DB.QueryRowContext(ctx, `SELECT inventory FROM player_profiles WHERE user_id=$1`, userID).Scan(&raw); e != nil {
		return nil, e
	}
	storage, e := parseStorage(raw)
	if e != nil {
		return nil, e
	}
	for key, entry := range storage {
		if !strings.HasPrefix(key, "resource_") || storageCount(entry) <= 0 || marketStackEntryRestricted(entry) {
			continue
		}
		id, e := strconv.Atoi(strings.TrimPrefix(key, "resource_"))
		if e != nil {
			continue
		}
		var name, image, category, subcategory string
        if e = DB.QueryRowContext(ctx, `SELECT type,COALESCE(image,''),COALESCE(market_category,''),COALESCE(market_subcategory,type) FROM resources WHERE id=$1 AND tradable`, id).Scan(&name, &image, &category, &subcategory); e != nil {
			continue
		}
		if category==""{category=marketResourceCategory(name)}
        items = append(items, MarketItem{Kind: "resource", ID: int64(id), Code: key, Name: name, Category: category, Subcategory: subcategory, Rarity: "gray", ImageURL: image, Level: 1, Tradable: true, Stackable:true, Quantity:storageCount(entry)})
	}
    stackRows,e:=DB.QueryContext(ctx,`SELECT inventory_key,catalog_id,code,name,category,subcategory,rarity,image_url FROM market_stack_catalog WHERE tradable AND stackable ORDER BY name`)
    if e!=nil{return nil,e};defer stackRows.Close()
    for stackRows.Next(){var key,code,name,category,sub,rarity,image string;var catalogID int
        if e=stackRows.Scan(&key,&catalogID,&code,&name,&category,&sub,&rarity,&image);e!=nil{return nil,e}
        entry:=storage[key];if storageCount(entry)<=0||marketStackEntryRestricted(entry)||marketStackVariantUnsupported(entry){continue}
        items=append(items,MarketItem{Kind:"stack",ID:int64(catalogID),StackKey:key,Code:code,Name:name,Category:category,Subcategory:sub,Rarity:rarity,ImageURL:image,Level:1,Tradable:true,Stackable:true,Quantity:storageCount(entry)})
    }
    return items,stackRows.Err()
}
const marketAuctionCategorySQL = "CASE WHEN a.item_kind='stack' THEN sc.category WHEN a.item_kind='resource' THEN "+marketResourceCategorySQL+" ELSE "+marketEquipmentCategorySQL+" END"
const marketOrderCategorySQL = "CASE WHEN o.item_kind='stack' THEN sc.category WHEN o.item_kind='resource' THEN "+marketResourceCategorySQL+" ELSE "+marketEquipmentCategorySQL+" END"
func MarketAuctions(ctx context.Context,userID int,scope string,f MarketFilter)([]MarketAuction,error){
 if err:=f.Validate();err!=nil{return nil,err};args:=[]any{};where:=[]string{"true"}
 if f.ItemKind!=""{where=append(where,"a.item_kind="+marketSQLArg(&args,f.ItemKind));if f.ItemKind=="stack"{where=append(where,"a.stack_key="+marketSQLArg(&args,f.ItemKey))}else if f.ItemKind=="resource"{where=append(where,"a.resource_id::text="+marketSQLArg(&args,f.ItemKey))}else{where=append(where,"ii.template_id::text="+marketSQLArg(&args,f.ItemKey))}}
 if scope=="related"{if f.TargetID<=0{return nil,ErrMarketInvalid};where=append(where,"a.id="+marketSQLArg(&args,f.TargetID),"(a.seller_user_id="+marketSQLArg(&args,userID)+" OR a.highest_bidder_user_id="+marketSQLArg(&args,userID)+" OR EXISTS(SELECT 1 FROM market_bids mb WHERE mb.auction_id=a.id AND mb.bidder_user_id="+marketSQLArg(&args,userID)+") OR EXISTS(SELECT 1 FROM market_transactions t WHERE t.auction_id=a.id AND t.buyer_user_id="+marketSQLArg(&args,userID)+"))")}else if scope=="mine"{where=append(where,"a.seller_user_id="+marketSQLArg(&args,userID))}else if scope=="bids"{where=append(where,"EXISTS(SELECT 1 FROM market_bids b WHERE b.auction_id=a.id AND b.bidder_user_id="+marketSQLArg(&args,userID)+")")}else{where=append(where,"a.status='active' AND a.expires_at>clock_timestamp()")}
 if f.Status!=""{if scope=="all"||f.Status!="active"&&f.Status!="sold"&&f.Status!="expired"&&f.Status!="cancelled"{return nil,ErrMarketInvalid};where=append(where,"a.status="+marketSQLArg(&args,f.Status))}
 if f.Search!=""{where=append(where,"lower(COALESCE(it.name,r.type,sc.name)) LIKE lower("+marketSQLArg(&args,"%"+f.Search+"%")+")")}
 if f.Category!=""{if f.Category=="equipment"{where=append(where,"a.item_kind='equipment'")}else{where=append(where,marketAuctionCategorySQL+"="+marketSQLArg(&args,f.Category))}}
 if f.Subcategory!=""{where=append(where,"COALESCE(it.item_type,NULLIF(r.market_subcategory,''),r.type,sc.subcategory)="+marketSQLArg(&args,f.Subcategory))}
 if f.Rarity!=""{where=append(where,"COALESCE(it.rarity,sc.rarity,'gray')="+marketSQLArg(&args,f.Rarity))}
 if f.Class!=""{where=append(where,"(it.class_restriction IS NULL OR it.class_restriction="+marketSQLArg(&args,f.Class)+")")}
 if f.MinLevel>0{where=append(where,"COALESCE(it.level_requirement,1)>="+marketSQLArg(&args,f.MinLevel))};if f.MaxLevel>0{where=append(where,"COALESCE(it.level_requirement,1)<="+marketSQLArg(&args,f.MaxLevel))}
 if f.MinPrice>0{where=append(where,"COALESCE(a.highest_bid,a.start_price)>="+marketSQLArg(&args,f.MinPrice))};if f.MaxPrice>0{where=append(where,"COALESCE(a.highest_bid,a.start_price)<="+marketSQLArg(&args,f.MaxPrice))}
 if f.Party!=""{where=append(where,"lower(p.name) LIKE lower("+marketSQLArg(&args,"%"+f.Party+"%")+")")}
 q:=`SELECT a.id,a.seller_user_id,p.name,a.item_kind,COALESCE(a.item_instance_id::text,''),COALESCE(a.stack_key,''),COALESCE(it.id,r.id,sc.catalog_id,0)::bigint,COALESCE(it.code,'resource_'||r.id,sc.code),COALESCE(it.name,r.type,sc.name),`+marketAuctionCategorySQL+`,COALESCE(it.item_type,NULLIF(r.market_subcategory,''),r.type,sc.subcategory),COALESCE(it.rarity,sc.rarity,'gray'),COALESCE(it.image_url,r.image,sc.image_url,''),COALESCE(it.level_requirement,1),it.class_restriction,`+marketAttributesSQL+`,ii.durability,ii.max_durability,COALESCE(sc.required_forge_level,0),a.quantity,a.start_price,a.buyout_price,a.highest_bid,a.bid_count,a.status,a.expires_at,a.created_at FROM market_auctions a JOIN player_profiles p ON p.user_id=a.seller_user_id LEFT JOIN item_instances ii ON ii.id=a.item_instance_id LEFT JOIN item_templates it ON it.id=ii.template_id LEFT JOIN resources r ON r.id=a.resource_id LEFT JOIN market_stack_catalog sc ON sc.inventory_key=a.stack_key WHERE `+strings.Join(where," AND ")+` ORDER BY `+marketOrderBy(f.Sort,true)+` LIMIT `+marketSQLArg(&args,f.Limit)+` OFFSET `+marketSQLArg(&args,(f.Page-1)*f.Limit)
 rows,e:=DB.QueryContext(ctx,q,args...);if e!=nil{return nil,e};defer rows.Close();out:=[]MarketAuction{}
 for rows.Next(){var a MarketAuction;var kind,stackKey,code,name,category,sub,rarity,image string;var itemID int64;var level int;var class sql.NullString;var buyout,bid sql.NullInt64;var attributes []byte;var durability,maxDurability sql.NullInt64
  e=rows.Scan(&a.ID,&a.SellerID,&a.Seller,&kind,&a.InstanceID,&stackKey,&itemID,&code,&name,&category,&sub,&rarity,&image,&level,&class,&attributes,&durability,&maxDurability,&a.Item.RequiredForgeLevel,&a.Quantity,&a.StartPrice,&buyout,&bid,&a.BidCount,&a.Status,&a.ExpiresAt,&a.CreatedAt);if e!=nil{return nil,e}
  requiredForge:=a.Item.RequiredForgeLevel;a.Item=MarketItem{InstanceID:a.InstanceID,Attributes:marketItemAttributes(attributes),RequiredForgeLevel:requiredForge,Kind:kind,ID:itemID,StackKey:stackKey,Code:code,Name:name,Category:category,Subcategory:sub,Rarity:rarity,ImageURL:image,Level:level,Class:class.String,Tradable:true,Stackable:kind!="equipment"};if durability.Valid{v:=int(durability.Int64);a.Item.Durability=&v};if maxDurability.Valid{v:=int(maxDurability.Int64);a.Item.MaxDurability=&v};if buyout.Valid{a.BuyoutPrice=&buyout.Int64};if bid.Valid{a.HighestBid=&bid.Int64};out=append(out,a)
 };return out,rows.Err()
}
func MarketOrders(ctx context.Context,userID int,scope string,f MarketFilter)([]MarketOrder,error){
 if err:=f.Validate();err!=nil{return nil,err};args:=[]any{};where:=[]string{"true"}
 if f.ItemKind!=""{where=append(where,"o.item_kind="+marketSQLArg(&args,f.ItemKind));if f.ItemKind=="stack"{where=append(where,"o.stack_key="+marketSQLArg(&args,f.ItemKey))}else if f.ItemKind=="resource"{where=append(where,"o.resource_id::text="+marketSQLArg(&args,f.ItemKey))}else{where=append(where,"o.template_id::text="+marketSQLArg(&args,f.ItemKey))}}
 if scope=="related"{if f.TargetID<=0{return nil,ErrMarketInvalid};where=append(where,"o.id="+marketSQLArg(&args,f.TargetID),"(o.buyer_user_id="+marketSQLArg(&args,userID)+" OR EXISTS(SELECT 1 FROM market_fills mf WHERE mf.order_id=o.id AND mf.seller_user_id="+marketSQLArg(&args,userID)+"))")}else if scope=="mine"{where=append(where,"o.buyer_user_id="+marketSQLArg(&args,userID))}else{where=append(where,"o.status IN ('open','partially_filled') AND o.expires_at>clock_timestamp()")}
 if f.Status!=""{if scope=="all"||f.Status!="open"&&f.Status!="partially_filled"&&f.Status!="filled"&&f.Status!="expired"&&f.Status!="cancelled"{return nil,ErrMarketInvalid};where=append(where,"o.status="+marketSQLArg(&args,f.Status))}
 if f.Search!=""{where=append(where,"lower(COALESCE(it.name,r.type,sc.name)) LIKE lower("+marketSQLArg(&args,"%"+f.Search+"%")+")")}
 if f.Category!=""{if f.Category=="equipment"{where=append(where,"o.item_kind='equipment'")}else{where=append(where,marketOrderCategorySQL+"="+marketSQLArg(&args,f.Category))}}
 if f.Subcategory!=""{where=append(where,"COALESCE(it.item_type,NULLIF(r.market_subcategory,''),r.type,sc.subcategory)="+marketSQLArg(&args,f.Subcategory))}
 if f.Rarity!=""{where=append(where,"COALESCE(it.rarity,sc.rarity,'gray')="+marketSQLArg(&args,f.Rarity))}
 if f.Class!=""{where=append(where,"(it.class_restriction IS NULL OR it.class_restriction="+marketSQLArg(&args,f.Class)+")")}
 if f.MinLevel>0{where=append(where,"COALESCE(it.level_requirement,1)>="+marketSQLArg(&args,f.MinLevel))};if f.MaxLevel>0{where=append(where,"COALESCE(it.level_requirement,1)<="+marketSQLArg(&args,f.MaxLevel))}
 if f.MinPrice>0{where=append(where,"o.price_per_unit >= "+marketSQLArg(&args,f.MinPrice))};if f.MaxPrice>0{where=append(where,"o.price_per_unit <= "+marketSQLArg(&args,f.MaxPrice))}
 if f.Party!=""{where=append(where,"lower(p.name) LIKE lower("+marketSQLArg(&args,"%"+f.Party+"%")+")")}
 q:=`SELECT o.id,o.buyer_user_id,p.name,o.item_kind,COALESCE(o.stack_key,''),COALESCE(it.id,r.id,sc.catalog_id,0)::bigint,COALESCE(it.code,'resource_'||r.id,sc.code),COALESCE(it.name,r.type,sc.name),`+marketOrderCategorySQL+`,COALESCE(it.item_type,NULLIF(r.market_subcategory,''),r.type,sc.subcategory),COALESCE(it.rarity,sc.rarity,'gray'),COALESCE(it.image_url,r.image,sc.image_url,''),COALESCE(it.level_requirement,1),it.class_restriction,`+marketAttributesSQL+`,COALESCE(sc.required_forge_level,0),o.quantity,o.filled_quantity,o.price_per_unit,o.reserved_remaining,o.status,o.expires_at,o.created_at FROM market_orders o JOIN player_profiles p ON p.user_id=o.buyer_user_id LEFT JOIN resources r ON r.id=o.resource_id LEFT JOIN item_templates it ON it.id=o.template_id LEFT JOIN market_stack_catalog sc ON sc.inventory_key=o.stack_key WHERE `+strings.Join(where," AND ")+` ORDER BY `+marketOrderBy(f.Sort,false)+` LIMIT `+marketSQLArg(&args,f.Limit)+` OFFSET `+marketSQLArg(&args,(f.Page-1)*f.Limit)
 rows,e:=DB.QueryContext(ctx,q,args...);if e!=nil{return nil,e};defer rows.Close();out:=[]MarketOrder{}
 for rows.Next(){var o MarketOrder;var kind,stackKey,code,name,category,sub,rarity,image string;var itemID int64;var level int;var class sql.NullString;var attributes []byte
  e=rows.Scan(&o.ID,&o.BuyerID,&o.Buyer,&kind,&stackKey,&itemID,&code,&name,&category,&sub,&rarity,&image,&level,&class,&attributes,&o.Item.RequiredForgeLevel,&o.Quantity,&o.FilledQuantity,&o.PricePerUnit,&o.ReservedRemaining,&o.Status,&o.ExpiresAt,&o.CreatedAt);if e!=nil{return nil,e}
  requiredForge:=o.Item.RequiredForgeLevel;o.RemainingQuantity=o.Quantity-o.FilledQuantity;o.Item=MarketItem{Attributes:marketItemAttributes(attributes),RequiredForgeLevel:requiredForge,Kind:kind,ID:itemID,StackKey:stackKey,Code:code,Name:name,Category:category,Subcategory:sub,Rarity:rarity,ImageURL:image,Level:level,Class:class.String,Tradable:true,Stackable:kind!="equipment"};out=append(out,o)
 };return out,rows.Err()
}
func MarketDeliveries(ctx context.Context,userID,page,limit int)([]MarketDelivery,error){
 q:=`SELECT d.id,d.item_kind,COALESCE(d.item_instance_id::text,''),COALESCE(d.stack_key,''),COALESCE(it.id,r.id,sc.catalog_id,0)::bigint,COALESCE(it.code,'resource_'||r.id,sc.code),COALESCE(it.name,r.type,sc.name),COALESCE(it.item_type,NULLIF(r.market_subcategory,''),r.type,sc.subcategory),COALESCE(it.rarity,sc.rarity,'gray'),COALESCE(it.image_url,r.image,sc.image_url,''),d.quantity,d.created_at,COALESCE(sc.category,''),COALESCE(r.market_category,'') FROM market_deliveries d LEFT JOIN item_instances ii ON ii.id=d.item_instance_id LEFT JOIN item_templates it ON it.id=ii.template_id LEFT JOIN resources r ON r.id=d.resource_id LEFT JOIN market_stack_catalog sc ON sc.inventory_key=d.stack_key WHERE d.recipient_user_id=$1 AND d.claimed_at IS NULL ORDER BY d.id DESC LIMIT $2 OFFSET $3`
 rows,e:=DB.QueryContext(ctx,q,userID,limit,(page-1)*limit);if e!=nil{return nil,e};defer rows.Close();out:=[]MarketDelivery{}
 for rows.Next(){var d MarketDelivery;var kind,stackKey,code,name,sub,rarity,image,stackCategory,resourceCategory string;var itemID int64
  e=rows.Scan(&d.ID,&kind,&d.InstanceID,&stackKey,&itemID,&code,&name,&sub,&rarity,&image,&d.Quantity,&d.CreatedAt,&stackCategory,&resourceCategory);if e!=nil{return nil,e}
  category:=marketCategory(sub,kind);if kind=="resource"{category=resourceCategory;if category==""{category=marketResourceCategory(sub)}};if kind=="stack"{category=stackCategory};d.Item=MarketItem{Kind:kind,ID:itemID,StackKey:stackKey,Code:code,Name:name,Category:category,Subcategory:sub,Rarity:rarity,ImageURL:image,Level:1,Tradable:true,Stackable:kind!="equipment"};out=append(out,d)
 };return out,rows.Err()
}
func MarketTransactions(ctx context.Context, userID, page, limit int) ([]MarketHistory, error) {
	rows, e := DB.QueryContext(ctx, `SELECT id,kind,buyer_user_id,seller_user_id,gross_price,fee,created_at FROM market_transactions WHERE buyer_user_id=$1 OR seller_user_id=$1 ORDER BY id DESC LIMIT $2 OFFSET $3`, userID, limit, (page-1)*limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []MarketHistory{}
	for rows.Next() {
		var h MarketHistory
		if e = rows.Scan(&h.ID, &h.Kind, &h.BuyerID, &h.SellerID, &h.Gross, &h.Fee, &h.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

type MarketWalletClaim struct {ID int64 `json:"id"`;Amount int64 `json:"amount"`;CreatedAt time.Time `json:"createdAt"`}
func MarketWalletClaims(ctx context.Context,userID,page,limit int)([]MarketWalletClaim,error){rows,e:=DB.QueryContext(ctx,`SELECT id,amount,created_at FROM market_wallet_claims WHERE recipient_user_id=$1 AND claimed_at IS NULL ORDER BY id DESC LIMIT $2 OFFSET $3`,userID,limit,(page-1)*limit);if e!=nil{return nil,e};defer rows.Close();out:=[]MarketWalletClaim{};for rows.Next(){var c MarketWalletClaim;if e=rows.Scan(&c.ID,&c.Amount,&c.CreatedAt);e!=nil{return nil,e};out=append(out,c)};return out,rows.Err()}

type MarketCategoryOption struct {Category string `json:"category"`;Subcategory string `json:"subcategory"`}
func MarketCategories(ctx context.Context)([]MarketCategoryOption,error){
 q:=`SELECT DISTINCT category,subcategory FROM (
 SELECT `+marketEquipmentCategorySQL+` category,it.item_type subcategory FROM item_templates it WHERE it.tradable
 UNION ALL SELECT `+marketResourceCategorySQL+`,COALESCE(NULLIF(r.market_subcategory,''),r.type) FROM resources r WHERE r.tradable
 UNION ALL SELECT category,subcategory FROM market_stack_catalog WHERE tradable
 ) c ORDER BY category,subcategory`
 rows,e:=DB.QueryContext(ctx,q);if e!=nil{return nil,e};defer rows.Close();options:=[]MarketCategoryOption{}
 for rows.Next(){var v MarketCategoryOption;if e=rows.Scan(&v.Category,&v.Subcategory);e!=nil{return nil,e};options=append(options,v)};return options,rows.Err()
}
