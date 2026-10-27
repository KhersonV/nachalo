package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"
)

var ErrMarketInvalid = errors.New("invalid market request")
var ErrMarketUnavailable = errors.New("market listing unavailable")
var ErrMarketForbidden = errors.New("market action forbidden")
var ErrMarketFunds = errors.New("not enough gold")
var ErrMarketInventory = errors.New("item unavailable")
var ErrMarketLimit = errors.New("market limit reached")
var ErrMarketIdempotency = errors.New("idempotency key reused for another action")

const MarketMaxPrice int64 = 1_000_000_000
const MarketMaxTotal int64 = 2_000_000_000
const MarketMaxQuantity = 9999
const MarketFeePercent = 1
const MarketMinBidIncrement int64 = 1
const MarketAuctionLimit = 3
const MarketOrderLimit = 3

func marketFee(gross int64) int64 { return (gross*MarketFeePercent + 99) / 100 }
func validMarketAmount(price int64, quantity int) bool {
	return price > 0 && price <= MarketMaxPrice && quantity > 0 && quantity <= MarketMaxQuantity && int64(quantity) <= MarketMaxTotal/price && price*int64(quantity) <= MarketMaxTotal && price>marketFee(price)
}
func marketTx(ctx context.Context) (*sql.Tx, error) {
	return DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
}
func marketIdempotency(ctx context.Context, tx *sql.Tx, userID int, key, action string) (int64, bool, error) {
	if key == "" {
		return 0, false, ErrMarketInvalid
	}
	var id int64
	err := tx.QueryRowContext(ctx, `INSERT INTO market_idempotency(user_id,request_key,action,result_id) VALUES($1,$2::uuid,$3,0) ON CONFLICT DO NOTHING RETURNING result_id`, userID, key, action).Scan(&id)
	if err == nil {
		return 0, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}
	var prior string
	if err = tx.QueryRowContext(ctx, `SELECT action,result_id FROM market_idempotency WHERE user_id=$1 AND request_key=$2::uuid`, userID, key).Scan(&prior, &id); err != nil {
		return 0, false, err
	}
	if prior != action {
		return 0, false, ErrMarketIdempotency
	}
	return id, true, nil
}
func marketResult(ctx context.Context, tx *sql.Tx, userID int, key string, id int64) error {
	_, err := tx.ExecContext(ctx, `UPDATE market_idempotency SET result_id=$3 WHERE user_id=$1 AND request_key=$2::uuid`, userID, key, id)
	return err
}
func marketProfiles(ctx context.Context, tx *sql.Tx, ids ...int) (map[int]int64, error) {
	uniq := map[int]bool{}
	for _, id := range ids {
		uniq[id] = true
	}
	ordered := make([]int, 0, len(uniq))
	for id := range uniq {
		ordered = append(ordered, id)
	}
	sort.Ints(ordered)
	balances := map[int]int64{}
	for _, id := range ordered {
		var balance int64
		if err := tx.QueryRowContext(ctx, `SELECT balance FROM player_profiles WHERE user_id=$1 FOR UPDATE`, id).Scan(&balance); err != nil {
			return nil, err
		}
		balances[id] = balance
	}
	return balances, nil
}
func marketDebit(ctx context.Context, tx *sql.Tx, id int, amount int64) error {
	if amount <= 0 || amount > MarketMaxTotal {
		return ErrMarketInvalid
	}
	r, e := tx.ExecContext(ctx, `UPDATE player_profiles SET balance=balance-$2,updated_at=now() WHERE user_id=$1 AND balance >= $2`, id, amount)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrMarketFunds
	}
	return nil
}
func marketCredit(ctx context.Context, tx *sql.Tx, id int, amount int64) error {
	if amount < 0 {
		return ErrMarketInvalid
	}
	r, e := tx.ExecContext(ctx, `UPDATE player_profiles SET balance=balance+$2,updated_at=now() WHERE user_id=$1 AND balance <= 2147483647-$2`, id, amount)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
        var claimID int64
        e = tx.QueryRowContext(ctx, `INSERT INTO market_wallet_claims(recipient_user_id,amount) VALUES($1,$2) RETURNING id`,id,amount).Scan(&claimID)
        if e!=nil{return e};return marketNotice(ctx,tx,id,fmt.Sprintf("wallet:%d",claimID),fmt.Sprintf("%d золота готово к получению.",amount),"deliveries",claimID)
    }
    return nil
}
func marketResourceKey(id int) string { return "resource_" + strconv.Itoa(id) }
func marketResourceAllowed(ctx context.Context, tx *sql.Tx, resourceID int) (bool, error) {
	var allowed bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM resources WHERE id=$1 AND tradable AND resource_kind='resource')`, resourceID).Scan(&allowed)
	return allowed, err
}
func marketTakeResource(ctx context.Context, tx *sql.Tx, userID, resourceID, quantity int) ([]byte, error) {
	allowed, err := marketResourceAllowed(ctx, tx, resourceID)
	if err != nil { return nil, err }
	if !allowed { return nil, ErrMarketInvalid }
	var raw []byte
	if err := tx.QueryRowContext(ctx, `SELECT inventory FROM player_profiles WHERE user_id=$1 FOR UPDATE`, userID).Scan(&raw); err != nil {
		return nil, err
	}
	items, err := parseStorage(raw)
	if err != nil {
		return nil, err
	}
	key := marketResourceKey(resourceID)
	entry := items[key]
	if storageCount(entry) < quantity || marketStackEntryRestricted(entry) {
		return nil, ErrMarketInventory
	}
	copyEntry := map[string]any{}
	for k, v := range entry {
		copyEntry[k] = v
	}
	copyEntry["item_count"] = quantity
	remaining := storageCount(entry) - quantity
	if remaining == 0 {
		delete(items, key)
	} else {
		entry["item_count"] = remaining
	}
	updated, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	asset, err := json.Marshal(copyEntry)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE player_profiles SET inventory=$2::jsonb,updated_at=now() WHERE user_id=$1`, userID, string(updated))
	return asset, err
}
func marketAddResource(ctx context.Context, tx *sql.Tx, userID, resourceID, quantity int, asset []byte) error {
	// Keep legacy map-object deliveries in escrow instead of adding them to Storage.
	allowed, err := marketResourceAllowed(ctx, tx, resourceID)
	if err != nil { return err }
	if !allowed { return ErrMarketInvalid }
	var raw []byte
	if err := tx.QueryRowContext(ctx, `SELECT inventory FROM player_profiles WHERE user_id=$1 FOR UPDATE`, userID).Scan(&raw); err != nil {
		return err
	}
	items, err := parseStorage(raw)
	if err != nil {
		return err
	}
	key := marketResourceKey(resourceID)
	existing := items[key]
	if existing == nil {
		if err = json.Unmarshal(asset, &existing); err != nil {
			return err
		}
		existing["item_count"] = 0
		items[key] = existing
	}
	count := storageCount(existing)
	if count+quantity > 1_000_000_000 {
		return ErrMarketLimit
	}
	existing["item_count"] = count + quantity
	updated, err := json.Marshal(items)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE player_profiles SET inventory=$2::jsonb,updated_at=now() WHERE user_id=$1`, userID, string(updated))
	return err
}
func marketDelivery(ctx context.Context, tx *sql.Tx, recipient int, kind string, instance, stackKey string, resourceID, quantity int, asset []byte) error {
 var id int64;var err error
 if kind=="equipment" {err=tx.QueryRowContext(ctx,`INSERT INTO market_deliveries(recipient_user_id,item_kind,item_instance_id,quantity) VALUES($1,'equipment',$2::uuid,1) RETURNING id`,recipient,instance).Scan(&id)
 } else if kind=="stack" {err=tx.QueryRowContext(ctx,`INSERT INTO market_deliveries(recipient_user_id,item_kind,stack_key,resource_data,quantity) VALUES($1,'stack',$2,$3::jsonb,$4) RETURNING id`,recipient,stackKey,string(asset),quantity).Scan(&id)
 } else {err=tx.QueryRowContext(ctx,`INSERT INTO market_deliveries(recipient_user_id,item_kind,resource_id,resource_data,quantity) VALUES($1,'resource',$2,$3::jsonb,$4) RETURNING id`,recipient,resourceID,string(asset),quantity).Scan(&id)}
 if err!=nil{return err}
 return marketNotice(ctx,tx,recipient,fmt.Sprintf("delivery:%d",id),"Предмет готов к получению.","deliveries",id)
}
func marketEquipmentAvailable(ctx context.Context, tx *sql.Tx, userID int, instance string) error {
	var owner int
	var status string
	var character sql.NullInt64
	var boundUser, boundCharacter sql.NullInt64
	var soul, locked, tradable, inMatch, hasEquipment bool
	err := tx.QueryRowContext(ctx, `SELECT ii.owner_user_id,ii.status,ii.current_character_id,ii.bound_to_user_id,ii.bound_to_character_id,ii.is_soulbound,ii.is_locked,(it.tradable AND NOT (`+commonEquipmentSQL("it")+`)),ii.match_instance_id IS NOT NULL,EXISTS(SELECT 1 FROM character_equipment ce WHERE ce.item_instance_id=ii.id) FROM item_instances ii JOIN item_templates it ON it.id=ii.template_id WHERE ii.id=$1::uuid FOR UPDATE OF ii`, instance).Scan(&owner, &status, &character, &boundUser, &boundCharacter, &soul, &locked, &tradable, &inMatch, &hasEquipment)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrMarketInventory
	}
	if err != nil {
		return err
	}
	if owner != userID {
		return ErrMarketForbidden
	}
	if status != "inventory" || character.Valid || boundUser.Valid || boundCharacter.Valid || soul || locked || !tradable || inMatch || hasEquipment {
		return ErrMarketInventory
	}
	return nil
}
func marketMoveEquipment(ctx context.Context, tx *sql.Tx, instance string, owner int, status string) error {
	_, e := tx.ExecContext(ctx, `UPDATE item_instances SET owner_user_id=$2,status=$3,current_character_id=NULL,version=version+1,updated_at=now() WHERE id=$1::uuid`, instance, owner, status)
	return e
}
func marketEvent(ctx context.Context, tx *sql.Tx, instance, kind string, from, to int) error {
	_, e := tx.ExecContext(ctx, `INSERT INTO item_instance_events(item_instance_id,event_type,from_user_id,to_user_id) VALUES($1::uuid,$2,$3,$4)`, instance, kind, from, to)
	return e
}

// MarketCreateAuction removes a stack or changes a unique item's authoritative
// status in the same transaction that creates the listing.
func MarketCreateAuction(ctx context.Context, userID int, key, kind, instance, stackKey string, resourceID, quantity int, start, buyout int64, duration int) (id int64, err error) {
	if !validMarketAmount(start, 1) || quantity < 1 || quantity > MarketMaxQuantity || (buyout != 0 && (!validMarketAmount(buyout, 1) || buyout < start)) || (duration != 6 && duration != 12 && duration != 24) {
		return 0, ErrMarketInvalid
	}
	if kind == "equipment" {
		if quantity != 1 || instance == "" {
			return 0, ErrMarketInvalid
		}
	} else if kind == "resource" { if resourceID <= 0 { return 0, ErrMarketInvalid } } else if kind == "stack" { if stackKey == "" { return 0, ErrMarketInvalid } } else { return 0, ErrMarketInvalid }
	tx, e := marketTx(ctx)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	if id, duplicate, e := marketIdempotency(ctx, tx, userID, key, "create_auction"); e != nil {
		return 0, e
	} else if duplicate {
		return id, tx.Commit()
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(711000,$1)`, userID); e != nil {
		return 0, e
	}
	var count int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM market_auctions WHERE seller_user_id=$1 AND status='active'`, userID).Scan(&count); e != nil {
		return 0, e
	}
	if count >= MarketAuctionLimit {
		return 0, ErrMarketLimit
	}
	var asset []byte
	if kind == "equipment" {
		if e = marketEquipmentAvailable(ctx, tx, userID, instance); e != nil {
			return 0, e
		}
		if e = marketMoveEquipment(ctx, tx, instance, userID, "listed"); e != nil {
			return 0, e
		}
		if e = marketEvent(ctx, tx, instance, "listed", userID, userID); e != nil {
			return 0, e
		}
	} else if kind == "stack" {
        if asset,e=marketTakeStack(ctx,tx,userID,stackKey,quantity);e!=nil{return 0,e}
    } else {
		if asset, e = marketTakeResource(ctx, tx, userID, resourceID, quantity); e != nil {return 0, e}
	}
	var assetValue any
 if kind == "resource" || kind == "stack" { assetValue = string(asset) }
 var buyoutValue any
	if buyout > 0 {
		buyoutValue = buyout
	}
	e = tx.QueryRowContext(ctx, `INSERT INTO market_auctions(seller_user_id,item_kind,item_instance_id,resource_id,stack_key,resource_data,quantity,start_price,buyout_price,expires_at) VALUES($1,$2,NULLIF($3,'')::uuid,NULLIF($4,0),NULLIF($5,''),$6::jsonb,$7,$8,$9,now()+$10::int*interval '1 hour') RETURNING id`, userID, kind, instance, resourceID, stackKey, assetValue, quantity, start, buyoutValue, duration).Scan(&id)
	if e != nil {
		return 0, e
	}
	if e = marketResult(ctx, tx, userID, key, id); e != nil {
		return 0, e
	}
	return id, tx.Commit()
}

type marketAuctionLock struct {
	ID                   int64
	Seller               int
	Kind, Instance, StackKey string
	ResourceID, Quantity int
	Asset                []byte
	Start                int64
	Buyout               sql.NullInt64
	Highest              sql.NullInt64
	Bidder               sql.NullInt64
	Status               string
	Expired              bool
}

func lockAuction(ctx context.Context, tx *sql.Tx, id int64) (a marketAuctionLock, err error) {
	err = tx.QueryRowContext(ctx, `SELECT id,seller_user_id,item_kind,COALESCE(item_instance_id::text,''),COALESCE(resource_id,0),COALESCE(stack_key,''),quantity,resource_data,start_price,buyout_price,highest_bid,highest_bidder_user_id,status,expires_at<=clock_timestamp() FROM market_auctions WHERE id=$1 FOR UPDATE`, id).Scan(&a.ID, &a.Seller, &a.Kind, &a.Instance, &a.ResourceID, &a.StackKey, &a.Quantity, &a.Asset, &a.Start, &a.Buyout, &a.Highest, &a.Bidder, &a.Status, &a.Expired)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrMarketUnavailable
	}
	return
}
func marketAuctionDeliver(ctx context.Context, tx *sql.Tx, a marketAuctionLock, recipient int) error {
	if a.Kind == "equipment" {
		if err := marketMoveEquipment(ctx, tx, a.Instance, recipient, "trade_locked"); err != nil {
			return err
		}
		if err := marketEvent(ctx, tx, a.Instance, "traded", a.Seller, recipient); err != nil {
			return err
		}
	}
	return marketDelivery(ctx, tx, recipient, a.Kind, a.Instance, a.StackKey, a.ResourceID, a.Quantity, a.Asset)
}
func marketAuctionReturn(ctx context.Context, tx *sql.Tx, a marketAuctionLock) error {
	if a.Kind == "equipment" {
		if err := marketMoveEquipment(ctx, tx, a.Instance, a.Seller, "trade_locked"); err != nil {
			return err
		}
		if err := marketEvent(ctx, tx, a.Instance, "unlisted", a.Seller, a.Seller); err != nil {
			return err
		}
	}
	return marketDelivery(ctx, tx, a.Seller, a.Kind, a.Instance, a.StackKey, a.ResourceID, a.Quantity, a.Asset)
}
func marketSettleAuction(ctx context.Context, tx *sql.Tx, a marketAuctionLock, recipient int, gross int64, refundBid bool) error {
	if a.Kind == "resource" {
		allowed, err := marketResourceAllowed(ctx, tx, a.ResourceID)
		if err != nil { return err }
		if !allowed { return ErrMarketUnavailable }
	}
	if recipient == a.Seller {
		return ErrMarketForbidden
	}
    if gross<=0||gross>MarketMaxTotal||gross<=marketFee(gross){return ErrMarketInvalid}
    if err:=marketPurchaseEligible(ctx,tx,recipient,a.Kind,a.StackKey);err!=nil{return err}
	var variantDurability,variantMax sql.NullInt64;var templateID int64
	if a.Kind == "equipment" {
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status,durability,max_durability,template_id FROM item_instances WHERE id=$1::uuid FOR UPDATE`, a.Instance).Scan(&status,&variantDurability,&variantMax,&templateID); err != nil {
			return err
		}
		if status != "listed" {
			return ErrMarketInventory
		}
	}
	ids := []int{a.Seller, recipient}
	if a.Bidder.Valid && refundBid {
		ids = append(ids, int(a.Bidder.Int64))
	}
	if _, err := marketProfiles(ctx, tx, ids...); err != nil {
		return err
	}
	if refundBid && a.Bidder.Valid {
		if err := marketCredit(ctx, tx, int(a.Bidder.Int64), a.Highest.Int64); err != nil {return err}
        if int(a.Bidder.Int64)!=recipient {if err:=marketNotice(ctx,tx,int(a.Bidder.Int64),fmt.Sprintf("auction:%d:buyout_outbid",a.ID),"Лот выкуплен другим игроком. Ваша ставка возвращена.","auction",a.ID);err!=nil{return err}}
	}
	fee := marketFee(gross)
	if err := marketCredit(ctx, tx, a.Seller, gross-fee); err != nil {
		return err
	}
	if err := marketAuctionDeliver(ctx, tx, a, recipient); err != nil {
		return err
	}
	name,err:=marketAuctionName(ctx,tx,a);if err!=nil{return err}
    if err=marketNotice(ctx,tx,a.Seller,fmt.Sprintf("auction:%d:sold",a.ID),fmt.Sprintf("Ваш лот «%s» продан за %d золота.",name,gross),"auction",a.ID);err!=nil{return err}
    verb:="выиграли";if refundBid{verb="выкупили"}
    if err=marketNotice(ctx,tx,recipient,fmt.Sprintf("auction:%d:bought",a.ID),fmt.Sprintf("Вы %s «%s» за %d золота.",verb,name,gross),"auction",a.ID);err!=nil{return err}
    if err=marketAuditTx(ctx,tx,recipient,"auction_sale","auction",a.ID,map[string]any{"seller":a.Seller,"gross":gross,"quantity":a.Quantity,"unitPrice":float64(gross)/float64(a.Quantity),"itemKind":a.Kind,"templateId":templateID,"resourceId":a.ResourceID,"stackKey":a.StackKey,"variantDurability":variantDurability.Int64,"variantMaxDurability":variantMax.Int64,"hasDurability":variantDurability.Valid,"hasMaxDurability":variantMax.Valid,"method":verb});err!=nil{return err}
	if _, err := tx.ExecContext(ctx, `UPDATE market_auctions SET status='sold',settled_at=now() WHERE id=$1`, a.ID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO market_transactions(kind,auction_id,buyer_user_id,seller_user_id,gross_price,fee,variant_durability,variant_max_durability,variant_recorded) VALUES('auction_sale',$1,$2,$3,$4,$5,$6,$7,true)`, a.ID, recipient, a.Seller, gross, fee, variantDurability, variantMax)
	return err
}
func MarketBid(ctx context.Context, userID int, key string, id, amount int64) (result int64, err error) {
	if !validMarketAmount(amount, 1) {
		return 0, ErrMarketInvalid
	}
	tx, e := marketTx(ctx)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	if result, duplicate, e := marketIdempotency(ctx, tx, userID, key, "bid"); e != nil {
		return 0, e
	} else if duplicate {
		return result, tx.Commit()
	}
	a, e := lockAuction(ctx, tx, id)
	if e != nil {
		return 0, e
	}
	if a.Kind == "resource" {
		allowed, checkErr := marketResourceAllowed(ctx, tx, a.ResourceID)
		if checkErr != nil { return 0, checkErr }
		if !allowed { return 0, ErrMarketUnavailable }
	}
	if a.Status != "active" || a.Expired {
		return 0, ErrMarketUnavailable
	}
	if a.Seller == userID {
		return 0, ErrMarketForbidden
	}
    if e=marketPurchaseEligible(ctx,tx,userID,a.Kind,a.StackKey);e!=nil{return 0,e}
	minimum := a.Start
	if a.Highest.Valid {
		minimum = a.Highest.Int64 + MarketMinBidIncrement
	}
	if amount < minimum {
		return 0, ErrMarketInvalid
	}
	if a.Buyout.Valid && amount >= a.Buyout.Int64 {
		return 0, ErrMarketInvalid
	}
	ids := []int{userID}
	if a.Bidder.Valid {
		ids = append(ids, int(a.Bidder.Int64))
	}
	if _, e = marketProfiles(ctx, tx, ids...); e != nil {
		return 0, e
	}
	debit := amount
	if a.Bidder.Valid && int(a.Bidder.Int64) == userID {
		debit -= a.Highest.Int64
	}
	if e = marketDebit(ctx, tx, userID, debit); e != nil {
		return 0, e
	}
	if a.Bidder.Valid && int(a.Bidder.Int64) != userID {
		if e = marketCredit(ctx, tx, int(a.Bidder.Int64), a.Highest.Int64); e != nil {return 0,e}
        name,nameErr:=marketAuctionName(ctx,tx,a);if nameErr!=nil{return 0,nameErr}
        if e=marketNotice(ctx,tx,int(a.Bidder.Int64),fmt.Sprintf("auction:%d:outbid:%d",a.ID,a.Highest.Int64),fmt.Sprintf("Вашу ставку на «%s» перебили.",name),"auction",a.ID);e!=nil{return 0,e}
	}
	if _, e = tx.ExecContext(ctx, `UPDATE market_auctions SET highest_bid=$2,highest_bidder_user_id=$3,bid_count=bid_count+1 WHERE id=$1`, id, amount, userID); e != nil {
		return 0, e
	}
	if e = tx.QueryRowContext(ctx, `INSERT INTO market_bids(auction_id,bidder_user_id,amount) VALUES($1,$2,$3) RETURNING id`, id, userID, amount).Scan(&result); e != nil {
		return 0, e
	}
	if e = marketResult(ctx, tx, userID, key, result); e != nil {
		return 0, e
	}
	return result, tx.Commit()
}
func MarketBuyout(ctx context.Context, userID int, key string, id int64) (result int64, err error) {
	tx, e := marketTx(ctx)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	if result, duplicate, e := marketIdempotency(ctx, tx, userID, key, "buyout"); e != nil {
		return 0, e
	} else if duplicate {
		return result, tx.Commit()
	}
	a, e := lockAuction(ctx, tx, id)
	if e != nil {
		return 0, e
	}
	if a.Kind == "resource" {
		allowed, checkErr := marketResourceAllowed(ctx, tx, a.ResourceID)
		if checkErr != nil { return 0, checkErr }
		if !allowed { return 0, ErrMarketUnavailable }
	}
	if a.Status != "active" || a.Expired || !a.Buyout.Valid {return 0,ErrMarketUnavailable};if !validMarketAmount(a.Buyout.Int64,1){return 0,ErrMarketInvalid}
	if a.Seller == userID {
		return 0, ErrMarketForbidden
	}
    if e=marketPurchaseEligible(ctx,tx,userID,a.Kind,a.StackKey);e!=nil{return 0,e}
	if a.Kind == "equipment" {
		var status string
		if e = tx.QueryRowContext(ctx, `SELECT status FROM item_instances WHERE id=$1::uuid FOR UPDATE`, a.Instance).Scan(&status); e != nil {
			return 0, e
		}
		if status != "listed" {
			return 0, ErrMarketInventory
		}
	}
	ids := []int{userID, a.Seller}
	if a.Bidder.Valid {
		ids = append(ids, int(a.Bidder.Int64))
	}
	if _, e = marketProfiles(ctx, tx, ids...); e != nil {
		return 0, e
	}
	if e = marketDebit(ctx, tx, userID, a.Buyout.Int64); e != nil {
		return 0, e
	}
	if e = marketSettleAuction(ctx, tx, a, userID, a.Buyout.Int64, true); e != nil {
		return 0, e
	}
	if e = marketResult(ctx, tx, userID, key, id); e != nil {
		return 0, e
	}
	return id, tx.Commit()
}
func MarketCancelAuction(ctx context.Context, userID int, key string, id int64) (result int64, err error) {
	tx, e := marketTx(ctx)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	if result, duplicate, e := marketIdempotency(ctx, tx, userID, key, "cancel_auction"); e != nil {
		return 0, e
	} else if duplicate {
		return result, tx.Commit()
	}
	a, e := lockAuction(ctx, tx, id)
	if e != nil {
		return 0, e
	}
	if a.Seller != userID {
		return 0, ErrMarketForbidden
	}
	if a.Status != "active" || a.Expired || a.Highest.Valid {
		return 0, ErrMarketUnavailable
	}
	if a.Kind == "equipment" {
		var status string
		if e = tx.QueryRowContext(ctx, `SELECT status FROM item_instances WHERE id=$1::uuid FOR UPDATE`, a.Instance).Scan(&status); e != nil {
			return 0, e
		}
		if status != "listed" {
			return 0, ErrMarketInventory
		}
	}
	if _, e = marketProfiles(ctx, tx, userID); e != nil {
		return 0, e
	}
	if e = marketAuctionReturn(ctx, tx, a); e != nil {return 0,e}
    name,nameErr:=marketAuctionName(ctx,tx,a);if nameErr!=nil{return 0,nameErr}
    if e=marketNotice(ctx,tx,a.Seller,fmt.Sprintf("auction:%d:cancelled",a.ID),fmt.Sprintf("Лот «%s» снят с продажи.",name),"auction",a.ID);e!=nil{return 0,e}
	if _, e = tx.ExecContext(ctx, `UPDATE market_auctions SET status='cancelled',settled_at=now() WHERE id=$1`, id); e != nil {
		return 0, e
	}
	if e = marketResult(ctx, tx, userID, key, id); e != nil {
		return 0, e
	}
	return id, tx.Commit()
}
func marketExpireAuction(ctx context.Context, id int64) error {
	tx, e := marketTx(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	a, e := lockAuction(ctx, tx, id)
	if e != nil {
		return e
	}
	if a.Status != "active" || !a.Expired {
		return tx.Commit()
	}
	invalidResource := false
	if a.Kind == "resource" {
		allowed, checkErr := marketResourceAllowed(ctx, tx, a.ResourceID)
		if checkErr != nil { return checkErr }
		invalidResource = !allowed
	}
	if a.Kind == "equipment" {
		var status string
		if e = tx.QueryRowContext(ctx, `SELECT status FROM item_instances WHERE id=$1::uuid FOR UPDATE`, a.Instance).Scan(&status); e != nil {
			return e
		}
		if status != "listed" {
			return ErrMarketInventory
		}
	}
	if a.Bidder.Valid {
        var eligibilityErr error
        if !invalidResource { eligibilityErr=marketPurchaseEligible(ctx,tx,int(a.Bidder.Int64),a.Kind,a.StackKey) }
        if invalidResource || eligibilityErr!=nil || a.Highest.Int64<=marketFee(a.Highest.Int64) {
            var requirement *MarketPurchaseRequirementError
			if eligibilityErr!=nil&&!errors.As(eligibilityErr,&requirement){return eligibilityErr}
            if _,e=marketProfiles(ctx,tx,a.Seller,int(a.Bidder.Int64));e!=nil{return e}
            if e=marketCredit(ctx,tx,int(a.Bidder.Int64),a.Highest.Int64);e!=nil{return e}
            if e=marketNotice(ctx,tx,int(a.Bidder.Int64),fmt.Sprintf("auction:%d:bid_refunded",a.ID),"Аукцион завершён без продажи. Ваша ставка возвращена.","auction",a.ID);e!=nil{return e}
            if e=marketAuctionReturn(ctx,tx,a);e!=nil{return e}
            _,e=tx.ExecContext(ctx,`UPDATE market_auctions SET status='expired',settled_at=now() WHERE id=$1`,id);if e==nil{e=marketNotice(ctx,tx,a.Seller,fmt.Sprintf("auction:%d:expired",a.ID),"Аукцион завершён без продажи. Предмет готов к получению.","auction",a.ID)}
        } else {e=marketSettleAuction(ctx, tx, a, int(a.Bidder.Int64), a.Highest.Int64, false)}
	} else {
		if _, e = marketProfiles(ctx, tx, a.Seller); e == nil {
			e = marketAuctionReturn(ctx, tx, a)
		}
		if e == nil {
			_, e = tx.ExecContext(ctx, `UPDATE market_auctions SET status='expired',settled_at=now() WHERE id=$1`, id);if e==nil{e=marketNotice(ctx,tx,a.Seller,fmt.Sprintf("auction:%d:expired",a.ID),"Аукцион завершён без продажи. Предмет готов к получению.","auction",a.ID)}
		}
	}
	if e != nil {
		return e
	}
	return tx.Commit()
}

func MarketCreateOrder(ctx context.Context, userID int, key string, resourceID, quantity int, price int64) (id int64, err error) {
	if resourceID <= 0 || !validMarketAmount(price, quantity) {
		return 0, ErrMarketInvalid
	}
	tx, e := marketTx(ctx)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	if id, duplicate, e := marketIdempotency(ctx, tx, userID, key, "create_order"); e != nil {
		return 0, e
	} else if duplicate {
		return id, tx.Commit()
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(711001,$1)`, userID); e != nil {
		return 0, e
	}
	var count int
	var exists bool
	if exists, e = marketResourceAllowed(ctx, tx, resourceID); e != nil {
		return 0, e
	}
	if !exists {
		return 0, ErrMarketInvalid
	}
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM market_orders WHERE buyer_user_id=$1 AND status IN ('open','partially_filled')`, userID).Scan(&count); e != nil {
		return 0, e
	}
	if count >= MarketOrderLimit {
		return 0, ErrMarketLimit
	}
	if _, e = marketProfiles(ctx, tx, userID); e != nil {
		return 0, e
	}
	total := price * int64(quantity)
	if e = marketDebit(ctx, tx, userID, total); e != nil {
		return 0, e
	}
	e = tx.QueryRowContext(ctx, `INSERT INTO market_orders(buyer_user_id,resource_id,quantity,price_per_unit,reserved_remaining,expires_at) VALUES($1,$2,$3,$4,$5,now()+interval '24 hours') RETURNING id`, userID, resourceID, quantity, price, total).Scan(&id)
	if e != nil {
		return 0, e
	}
	if e = marketResult(ctx, tx, userID, key, id); e != nil {
		return 0, e
	}
	return id, tx.Commit()
}

type marketOrderLock struct {
	ID                                  int64
	Buyer, ResourceID, Quantity, Filled int
 Kind, StackKey string
 TemplateID int64
	Price, Reserved                     int64
	Status                              string
	Expired                             bool
}

func lockOrder(ctx context.Context, tx *sql.Tx, id int64) (o marketOrderLock, err error) {
	err = tx.QueryRowContext(ctx, `SELECT id,buyer_user_id,COALESCE(resource_id,0),quantity,filled_quantity,price_per_unit,reserved_remaining,status,expires_at<=clock_timestamp(),item_kind,COALESCE(template_id,0),COALESCE(stack_key,'') FROM market_orders WHERE id=$1 FOR UPDATE`, id).Scan(&o.ID, &o.Buyer, &o.ResourceID, &o.Quantity, &o.Filled, &o.Price, &o.Reserved, &o.Status, &o.Expired, &o.Kind, &o.TemplateID, &o.StackKey)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrMarketUnavailable
	}
	return
}
func MarketFulfillOrder(ctx context.Context, userID int, key string, id int64, quantity int, itemInstanceID string) (result int64, err error) {
	if quantity <= 0 || quantity > MarketMaxQuantity {
		return 0, ErrMarketInvalid
	}
	tx, e := marketTx(ctx)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	if result, duplicate, e := marketIdempotency(ctx, tx, userID, key, "fulfill_order"); e != nil {
		return 0, e
	} else if duplicate {
		return result, tx.Commit()
	}
	o, e := lockOrder(ctx, tx, id)
	if e != nil {
		return 0, e
	}
	if o.Kind == "resource" {
		allowed, checkErr := marketResourceAllowed(ctx, tx, o.ResourceID)
		if checkErr != nil { return 0, checkErr }
		if !allowed { return 0, ErrMarketUnavailable }
	}
	if o.Buyer == userID {
		return 0, ErrMarketForbidden
	}
	if o.Expired || (o.Status != "open" && o.Status != "partially_filled") || quantity > o.Quantity-o.Filled {
		return 0, ErrMarketUnavailable
	}
    if eligibilityErr:=marketPurchaseEligible(ctx,tx,o.Buyer,o.Kind,o.StackKey);eligibilityErr!=nil {
        var requirement *MarketPurchaseRequirementError
        if !errors.As(eligibilityErr,&requirement){return 0,eligibilityErr}
        if _,e=marketProfiles(ctx,tx,o.Buyer);e!=nil{return 0,e}
        if e=marketCredit(ctx,tx,o.Buyer,o.Reserved);e!=nil{return 0,e}
        if e=marketNotice(ctx,tx,o.Buyer,fmt.Sprintf("order:%d:cancelled",o.ID),"Заявка отменена: требование для покупки больше не выполнено. Резерв возвращён.","order",o.ID);e!=nil{return 0,e}
        if _,e=tx.ExecContext(ctx,`UPDATE market_orders SET status='cancelled',reserved_remaining=0,settled_at=now() WHERE id=$1`,id);e!=nil{return 0,e}
        if _,e=tx.ExecContext(ctx,`DELETE FROM market_idempotency WHERE user_id=$1 AND request_key=$2::uuid`,userID,key);e!=nil{return 0,e}
        if e=tx.Commit();e!=nil{return 0,e};return 0,eligibilityErr
    }
	var asset []byte
    if o.Kind == "equipment" {
        if quantity != 1 || itemInstanceID == "" { return 0,ErrMarketInvalid }
        if e=marketEquipmentAvailable(ctx,tx,userID,itemInstanceID);e!=nil{return 0,e}
        var templateID int64;var fullDurability bool
        if e=tx.QueryRowContext(ctx,`SELECT template_id,(durability IS NULL AND max_durability IS NULL) FROM item_instances WHERE id=$1::uuid`,itemInstanceID).Scan(&templateID,&fullDurability);e!=nil{return 0,e}
        if templateID!=o.TemplateID||!fullDurability{return 0,ErrMarketInventory}
    }
    if _, e = marketProfiles(ctx, tx, userID, o.Buyer); e != nil {return 0,e}
    if o.Kind == "stack" {asset,e=marketTakeStack(ctx,tx,userID,o.StackKey,quantity)} else if o.Kind == "resource" {asset,e=marketTakeResource(ctx,tx,userID,o.ResourceID,quantity)}
    if e!=nil{return 0,e}
	gross := o.Price * int64(quantity)
    if gross<=0||gross>MarketMaxTotal||gross<=marketFee(gross){return 0,ErrMarketInvalid}
	fee := marketFee(gross)
	if e = marketCredit(ctx, tx, userID, gross-fee); e != nil {
		return 0, e
	}
    if o.Kind == "equipment" {
        if e=marketMoveEquipment(ctx,tx,itemInstanceID,o.Buyer,"trade_locked");e!=nil{return 0,e}
        if e=marketEvent(ctx,tx,itemInstanceID,"traded",userID,o.Buyer);e!=nil{return 0,e}
    }
    if e=marketDelivery(ctx,tx,o.Buyer,o.Kind,itemInstanceID,o.StackKey,o.ResourceID,quantity,asset);e!=nil{return 0,e}
	status := "partially_filled"
	if o.Filled+quantity == o.Quantity {
		status = "filled"
	}
	if _, e = tx.ExecContext(ctx, `UPDATE market_orders SET filled_quantity=filled_quantity+$2,reserved_remaining=reserved_remaining-$3,status=$4,settled_at=CASE WHEN $4='filled' THEN now() ELSE settled_at END WHERE id=$1`, id, quantity, gross, status); e != nil {
		return 0, e
	}
	if e = tx.QueryRowContext(ctx, `INSERT INTO market_fills(order_id,seller_user_id,quantity,gross_price,fee) VALUES($1,$2,$3,$4,$5) RETURNING id`, id, userID, quantity, gross, fee).Scan(&result); e != nil {
		return 0, e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO market_transactions(kind,fill_id,buyer_user_id,seller_user_id,gross_price,fee,variant_recorded) VALUES('exchange_fill',$1,$2,$3,$4,$5,true)`, result, o.Buyer, userID, gross, fee); e != nil {
		return 0, e
	}
	name,nameErr:=marketOrderName(ctx,tx,o);if nameErr!=nil{return 0,nameErr}
    if e=marketNotice(ctx,tx,o.Buyer,fmt.Sprintf("order:%d:fill:%d",o.ID,result),fmt.Sprintf("Заявка «%s» выполнена: %d/%d.",name,o.Filled+quantity,o.Quantity),"order",o.ID);e!=nil{return 0,e}
    if e=marketAuditTx(ctx,tx,userID,"exchange_fill","order",o.ID,map[string]any{"buyer":o.Buyer,"gross":gross,"quantity":quantity,"unitPrice":o.Price,"itemKind":o.Kind,"resourceId":o.ResourceID,"stackKey":o.StackKey,"templateId":o.TemplateID});e!=nil{return 0,e}
    if e = marketResult(ctx, tx, userID, key, result); e != nil {return 0,e}
	return result, tx.Commit()
}
func MarketCancelOrder(ctx context.Context, userID int, key string, id int64) (result int64, err error) {
	tx, e := marketTx(ctx)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	if result, duplicate, e := marketIdempotency(ctx, tx, userID, key, "cancel_order"); e != nil {
		return 0, e
	} else if duplicate {
		return result, tx.Commit()
	}
	o, e := lockOrder(ctx, tx, id)
	if e != nil {
		return 0, e
	}
	if o.Buyer != userID {
		return 0, ErrMarketForbidden
	}
	if o.Expired || (o.Status != "open" && o.Status != "partially_filled") {
		return 0, ErrMarketUnavailable
	}
	if _, e = marketProfiles(ctx, tx, userID); e != nil {
		return 0, e
	}
	if e = marketCredit(ctx, tx, userID, o.Reserved); e != nil {
		return 0, e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE market_orders SET status='cancelled',reserved_remaining=0,settled_at=now() WHERE id=$1`, id); e != nil {return 0,e}
    if e=marketNotice(ctx,tx,o.Buyer,fmt.Sprintf("order:%d:cancelled",o.ID),"Заявка отменена. Неиспользованный резерв возвращён.","order",o.ID);e!=nil{return 0,e}
	if e = marketResult(ctx, tx, userID, key, id); e != nil {
		return 0, e
	}
	return id, tx.Commit()
}
func marketExpireOrder(ctx context.Context, id int64) error {
	tx, e := marketTx(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	o, e := lockOrder(ctx, tx, id)
	if e != nil {
		return e
	}
	if (o.Status != "open" && o.Status != "partially_filled") || !o.Expired {
		return tx.Commit()
	}
	if _, e = marketProfiles(ctx, tx, o.Buyer); e != nil {
		return e
	}
	if e = marketCredit(ctx, tx, o.Buyer, o.Reserved); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE market_orders SET status='expired',reserved_remaining=0,settled_at=now() WHERE id=$1`, id); e != nil {return e}
    if e=marketNotice(ctx,tx,o.Buyer,fmt.Sprintf("order:%d:expired",o.ID),"Заявка истекла. Неиспользованный резерв возвращён.","order",o.ID);e!=nil{return e}
	return tx.Commit()
}

// MarketExpireDue is called by a server worker and before market reads. Its
// row locks make races with bids, buyouts and fills resolve in commit order.
func MarketExpireDue(ctx context.Context, limit int) error {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, e := DB.QueryContext(ctx, `SELECT id FROM market_auctions WHERE status='active' AND expires_at<=clock_timestamp() ORDER BY expires_at,id LIMIT $1`, limit)
	if e != nil {
		return e
	}
	auctionIDs := []int64{}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			break
		}
		auctionIDs = append(auctionIDs, id)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range auctionIDs {
		if e = marketExpireAuction(ctx, id); e != nil {
			return fmt.Errorf("expire auction %d: %w", id, e)
		}
	}
	rows, e = DB.QueryContext(ctx, `SELECT id FROM market_orders WHERE status IN ('open','partially_filled') AND expires_at<=clock_timestamp() ORDER BY expires_at,id LIMIT $1`, limit)
	if e != nil {
		return e
	}
	orderIDs := []int64{}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			break
		}
		orderIDs = append(orderIDs, id)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range orderIDs {
		if e = marketExpireOrder(ctx, id); e != nil {
			return fmt.Errorf("expire order %d: %w", id, e)
		}
	}
	_,e=DB.ExecContext(ctx,`DELETE FROM market_rate_events WHERE created_at<now()-interval '1 day'`);return e
}
func StartMarketExpiryWorker(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			runCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
			e := MarketExpireDue(runCtx, 100)
			cancel()
			if e != nil {
				fmt.Printf("market expiry: %v\n", e)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// Claimed resources merge into the permanent profile inventory. Equipment
// preserves its UUID and attributes throughout escrow and delivery.
func MarketClaimDelivery(ctx context.Context, userID int, key string, id int64) (result int64, err error) {
	tx, e := marketTx(ctx)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	if result, duplicate, e := marketIdempotency(ctx, tx, userID, key, "claim_delivery"); e != nil {
		return 0, e
	} else if duplicate {
		return result, tx.Commit()
	}
	var recipient, resourceID, quantity int
	var kind, instance, stackKey string
	var asset []byte
	var claimed sql.NullTime
	e = tx.QueryRowContext(ctx, `SELECT recipient_user_id,item_kind,COALESCE(item_instance_id::text,''),COALESCE(resource_id,0),COALESCE(stack_key,''),quantity,resource_data,claimed_at FROM market_deliveries WHERE id=$1 FOR UPDATE`, id).Scan(&recipient, &kind, &instance, &resourceID, &stackKey, &quantity, &asset, &claimed)
	if errors.Is(e, sql.ErrNoRows) {
		return 0, ErrMarketUnavailable
	}
	if e != nil {
		return 0, e
	}
	if recipient != userID {
		return 0, ErrMarketForbidden
	}
	if claimed.Valid {
		return 0, ErrMarketUnavailable
	}
	if kind == "equipment" {
		var owner int
		var status string
		if e = tx.QueryRowContext(ctx, `SELECT owner_user_id,status FROM item_instances WHERE id=$1::uuid FOR UPDATE`, instance).Scan(&owner, &status); e != nil {
			return 0, e
		}
		if owner != userID || status != "trade_locked" {
			return 0, ErrMarketInventory
		}
		if e = marketMoveEquipment(ctx, tx, instance, userID, "inventory"); e != nil {
			return 0, e
		}
    } else if kind == "stack" {
        if e=marketAddStack(ctx,tx,userID,stackKey,quantity,asset);e!=nil{return 0,e}
    } else {
        if e=marketAddResource(ctx,tx,userID,resourceID,quantity,asset);e!=nil{return 0,e}
    }
	if _, e = tx.ExecContext(ctx, `UPDATE market_deliveries SET claimed_at=now() WHERE id=$1`, id); e != nil {
		return 0, e
	}
	if e = marketResult(ctx, tx, userID, key, id); e != nil {
		return 0, e
	}
	return id, tx.Commit()
}

// MarketClaimGold moves a deferred credit into the existing profile wallet once
// there is room. The claim row is locked so retries cannot credit it twice.
func MarketClaimGold(ctx context.Context,userID int,key string,id int64)(result int64,err error){
 tx,e:=marketTx(ctx);if e!=nil{return 0,e};defer tx.Rollback()
 if result,duplicate,e:=marketIdempotency(ctx,tx,userID,key,"claim_gold");e!=nil{return 0,e}else if duplicate{return result,tx.Commit()}
 var recipient int;var amount int64;var claimed sql.NullTime
 e=tx.QueryRowContext(ctx,`SELECT recipient_user_id,amount,claimed_at FROM market_wallet_claims WHERE id=$1 FOR UPDATE`,id).Scan(&recipient,&amount,&claimed)
 if errors.Is(e,sql.ErrNoRows){return 0,ErrMarketUnavailable};if e!=nil{return 0,e};if recipient!=userID{return 0,ErrMarketForbidden};if claimed.Valid{return 0,ErrMarketUnavailable}
 if _,e=marketProfiles(ctx,tx,userID);e!=nil{return 0,e}
 var balance int64;if e=tx.QueryRowContext(ctx,`SELECT balance FROM player_profiles WHERE user_id=$1`,userID).Scan(&balance);e!=nil{return 0,e}
 if balance+amount>2147483647{return 0,ErrMarketLimit}
 if _,e=tx.ExecContext(ctx,`UPDATE player_profiles SET balance=balance+$2,updated_at=now() WHERE user_id=$1`,userID,amount);e!=nil{return 0,e}
 if _,e=tx.ExecContext(ctx,`UPDATE market_wallet_claims SET claimed_at=now() WHERE id=$1`,id);e!=nil{return 0,e}
 if e=marketResult(ctx,tx,userID,key,id);e!=nil{return 0,e};return id,tx.Commit()
}
