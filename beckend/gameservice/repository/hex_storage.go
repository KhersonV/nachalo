package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

var ErrLoadoutUnavailable = errors.New("loadout item unavailable in Storage")
var ErrLoadoutInvalid = errors.New("invalid HEX loadout")
var ErrStorageInsufficientGold = errors.New("insufficient balance")

type StorageState struct {
	PocketCapacity int                       `json:"pocketCapacity"`
	PocketsUsed    int                       `json:"pocketsUsed"`
	PocketStacks   map[string]int            `json:"pocketStacks"`
	Inventory      map[string]map[string]any `json:"inventory"`
	Loadout        map[string]int            `json:"loadout"`
	Balance        int                       `json:"balance"`
	Eligible       []string                  `json:"eligibleKeys"`
	Equipment      []StorageEquipment        `json:"equipment"`
}

type StorageEquipment struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

type storageRowsQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func listPermanentEquipment(ctx context.Context, query storageRowsQuerier, userID int) ([]StorageEquipment, error) {
	rows, err := query.QueryContext(ctx, `SELECT it.name,it.item_type,ii.status FROM item_instances ii JOIN item_templates it ON it.id=ii.template_id WHERE ii.owner_user_id=$1 AND (ii.match_instance_id IS NULL OR ii.status='equipped') AND ii.status <> 'deleted' ORDER BY it.name,ii.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	equipment := []StorageEquipment{}
	for rows.Next() {
		var item StorageEquipment
		if err := rows.Scan(&item.Name, &item.Type, &item.Status); err != nil {
			return nil, err
		}
		equipment = append(equipment, item)
	}
	return equipment, rows.Err()
}

func parseStorage(raw []byte) (map[string]map[string]any, error) {
	items := map[string]map[string]any{}
	if len(raw) == 0 || string(raw) == "null" {
		return items, nil
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("decode Storage: %w", err)
	}
	if items == nil {
		return map[string]map[string]any{}, nil
	}
	return items, nil
}

func parseLoadout(raw []byte) (map[string]int, error) {
	selected := map[string]int{}
	if len(raw) == 0 || string(raw) == "null" {
		return selected, nil
	}
	if err := json.Unmarshal(raw, &selected); err != nil {
		return nil, fmt.Errorf("decode HEX loadout: %w", err)
	}
	if selected == nil {
		return map[string]int{}, nil
	}
	return selected, nil
}

func storageCount(entry map[string]any) int {
	if entry == nil {
		return 0
	}
	switch value := entry["item_count"].(type) {
	case float64:
		if value >= 0 && value <= 1_000_000_000 && value == float64(int(value)) {
			return int(value)
		}
	case int:
		if value >= 0 {
			return value
		}
	case string:
		count, err := strconv.Atoi(value)
		if err == nil && count >= 0 {
			return count
		}
	}
	return 0
}

func transferLoadout(storage map[string]map[string]any, selected map[string]int) (map[string]map[string]any, error) {
	match := map[string]map[string]any{}
	for key, count := range selected {
		if count <= 0 || count > 9999 {
			return nil, ErrLoadoutInvalid
		}
		entry := storage[key]
		if storageCount(entry) < count {
			return nil, ErrLoadoutUnavailable
		}
		copyEntry := make(map[string]any, len(entry))
		for field, value := range entry {
			copyEntry[field] = value
		}
		copyEntry["item_count"] = count
		match[key] = copyEntry
		remaining := storageCount(entry) - count
		if remaining == 0 {
			delete(storage, key)
		} else {
			entry["item_count"] = remaining
		}
	}
	return match, nil
}

// The catalog, rather than a client supplied category, decides what may enter a match.
type storageQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func allowedHexLoadoutKeyTx(ctx context.Context, tx storageQuerier, key string) bool {
	if strings.HasPrefix(key, "blueprint_") {
		switch key {
		case "blueprint_scout_tower", "blueprint_turret", "blueprint_wall":
			return true
		}
		return false
	}
	if strings.HasPrefix(key, "scroll_") {
		id, err := strconv.Atoi(strings.TrimPrefix(key, "scroll_"))
		return err == nil && id >= 2001 && id <= 2006
	}
	if !strings.HasPrefix(key, "resource_") {
		return false
	}
	id, err := strconv.Atoi(strings.TrimPrefix(key, "resource_"))
	if err != nil || id <= 0 {
		return false
	}
	var resourceType, effect string
	if err := tx.QueryRowContext(ctx, `SELECT type,COALESCE(effect,'{}') FROM resources WHERE id=$1 AND resource_kind='resource'`, id).Scan(&resourceType, &effect); err != nil {
		return false
	}
	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	if resourceType == "wood" || resourceType == "stone" || resourceType == "iron" {
		return false
	}
	if recipe, ok := craftRecipe(resourceType); ok {
		return recipe.Effect != ""
	}
	if resourceType == "food" || resourceType == "water" || resourceType == "elixir" {
		return true
	}
	var effects map[string]any
	if json.Unmarshal([]byte(effect), &effects) != nil {
		return false
	}
	_, heals := effects["health"]
	_, energizes := effects["energy"]
	_, restoresAP := effects["ap_half"]
	return heals || energizes || restoresAP
}

func eligibleStorageKeys(ctx context.Context, query storageQuerier, storage map[string]map[string]any) []string {
	keys := make([]string, 0)
	for key, entry := range storage {
		if storageCount(entry) > 0 && allowedHexLoadoutKeyTx(ctx, query, key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func validateLoadoutTx(ctx context.Context, tx *sql.Tx, storage map[string]map[string]any, selected map[string]int) error {
	for key, count := range selected {
		if count <= 0 || count > 9999 || !allowedHexLoadoutKeyTx(ctx, tx, key) {
			return ErrLoadoutInvalid
		}
		if storageCount(storage[key]) < count {
			return ErrLoadoutUnavailable
		}
	}
	return nil
}

func GetHexStorage(ctx context.Context, userID int) (StorageState, error) {
	var state StorageState
	var inventoryRaw, loadoutRaw []byte
	err := DB.QueryRowContext(ctx, `SELECT inventory,hex_loadout,balance FROM player_profiles WHERE user_id=$1`, userID).Scan(&inventoryRaw, &loadoutRaw, &state.Balance)
	if err != nil {
		return state, err
	}
	state.Inventory, err = parseStorage(inventoryRaw)
	if err != nil {
		return state, err
	}
	if err := enrichProfessionStorageItems(ctx, DB, state.Inventory); err != nil {
		return state, err
	}
	state.Loadout, err = parseLoadout(loadoutRaw)
	if err == nil {
		state.Eligible = eligibleStorageKeys(ctx, DB, state.Inventory)
		state.Equipment, err = listPermanentEquipment(ctx, DB, userID)
	}
	if err == nil {
		err = enrichStoragePockets(ctx, DB, userID, &state)
	}
	return state, err
}

// Saving a selection never moves items. Queue cancellation therefore needs no refund.
func SaveHexLoadout(ctx context.Context, userID int, selected map[string]int) (StorageState, error) {
	var state StorageState
	if selected == nil {
		selected = map[string]int{}
	}
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return state, err
	}
	defer tx.Rollback()
	var inventoryRaw []byte
	if err = tx.QueryRowContext(ctx, `SELECT inventory,balance FROM player_profiles WHERE user_id=$1 FOR UPDATE`, userID).Scan(&inventoryRaw, &state.Balance); err != nil {
		return state, err
	}
	state.Inventory, err = parseStorage(inventoryRaw)
	if err != nil {
		return state, err
	}
	if err = validateLoadoutTx(ctx, tx, state.Inventory, selected); err != nil {
		return state, err
	}
	if err = validatePocketsTx(ctx, tx, userID, 0, selected); err != nil {
		return state, err
	}
	raw, err := json.Marshal(selected)
	if err != nil {
		return state, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE player_profiles SET hex_loadout=$2,updated_at=now() WHERE user_id=$1`, userID, raw); err != nil {
		return state, err
	}
	state.Loadout = selected
	state.Eligible = eligibleStorageKeys(ctx, tx, state.Inventory)
	state.Equipment, err = listPermanentEquipment(ctx, tx, userID)
	if err != nil {
		return state, err
	}
	if err = enrichStoragePockets(ctx, tx, userID, &state); err != nil {
		return state, err
	}
	if err = tx.Commit(); err != nil {
		return state, err
	}
	return state, nil
}

type hexMatchLoadout struct {
	Inventory map[string]map[string]any
	Balance   int
}

// Called only within CreateHexMatch. Sorting profile locks prevents team order deadlocks.
func takeHexLoadoutsTx(ctx context.Context, tx *sql.Tx, playerIDs []int, characterMaps ...map[int]int) (map[int]hexMatchLoadout, error) {
	ids := append([]int(nil), playerIDs...)
	sort.Ints(ids)
	result := make(map[int]hexMatchLoadout, len(ids))
	for _, id := range ids {
		var inventoryRaw, loadoutRaw []byte
		var balance int
		if err := tx.QueryRowContext(ctx, `SELECT inventory,hex_loadout,balance FROM player_profiles WHERE user_id=$1 FOR UPDATE`, id).Scan(&inventoryRaw, &loadoutRaw, &balance); err != nil {
			return nil, err
		}
		// The profile lock serializes concurrent HEX creation for this user.
		// Finished and defeated HEX participants retain their historical rows.
		var playing bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM match_players p JOIN matches m ON m.instance_id=p.instance_id WHERE p.user_id=$1 AND m.finished_at IS NULL AND COALESCE(m.mode,'')<>'hex-test' AND (COALESCE(m.geometry->>'kind','')<>'hex' OR p.health>0))`, id).Scan(&playing); err != nil {
			return nil, err
		}
		if playing {
			return nil, ErrCommandRejected
		}
		storage, err := parseStorage(inventoryRaw)
		if err != nil {
			return nil, err
		}
		selected, err := parseLoadout(loadoutRaw)
		if err != nil {
			return nil, err
		}
		if err := validateLoadoutTx(ctx, tx, storage, selected); err != nil {
			return nil, err
		}
		character := 0
		if len(characterMaps) > 0 {
			character = characterMaps[0][id]
		}
		if err := validatePocketsTx(ctx, tx, id, character, selected); err != nil {
			return nil, err
		}
		match, err := transferLoadout(storage, selected)
		if err != nil {
			return nil, err
		}
		updated, err := json.Marshal(storage)
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE player_profiles SET inventory=$2,hex_loadout='{}'::jsonb,updated_at=now() WHERE user_id=$1`, id, updated); err != nil {
			return nil, err
		}
		result[id] = hexMatchLoadout{Inventory: match, Balance: balance}
	}
	return result, nil
}

// Shop purchases serialize with loadout transfer and base construction on the profile row.
func PurchaseStorageItem(ctx context.Context, userID int, key string, count, totalCost int, item map[string]any, commandIDs ...string) error {
	if userID <= 0 || key == "" || count <= 0 || totalCost < 0 {
		return ErrLoadoutInvalid
	}
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if strings.HasPrefix(key, "resource_") {
		id, parseErr := strconv.Atoi(strings.TrimPrefix(key, "resource_"))
		if parseErr != nil || id <= 0 {
			return ErrLoadoutInvalid
		}
		var allowed bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM resources WHERE id=$1 AND resource_kind='resource')`, id).Scan(&allowed); err != nil {
			return err
		}
		if !allowed {
			return ErrLoadoutInvalid
		}
	}
	var inventoryRaw []byte
	var balance int
	if err := tx.QueryRowContext(ctx, `SELECT inventory,balance FROM player_profiles WHERE user_id=$1 FOR UPDATE`, userID).Scan(&inventoryRaw, &balance); err != nil {
		return err
	}
 command := ""
 if len(commandIDs)>0 { command=commandIDs[0] }
 if command!="" {
  if len(command)<8 || len(command)>128 { return ErrLoadoutInvalid }
  var previousKey string;var previousCount,previousCost int
  e:=tx.QueryRowContext(ctx,`SELECT item_key,quantity,total_cost FROM storage_purchase_operations WHERE user_id=$1 AND command_id=$2`,userID,command).Scan(&previousKey,&previousCount,&previousCost)
  if e==nil { if previousKey!=key || previousCount!=count || previousCost!=totalCost { return ErrCommandConflict }; return nil }
  if e!=sql.ErrNoRows {return e}
 }
	if balance < totalCost {
		return ErrStorageInsufficientGold
	}
	storage, err := parseStorage(inventoryRaw)
	if err != nil {
		return err
	}
	entry := storage[key]
	if entry == nil {
		entry = map[string]any{}
	}
	for field, value := range item {
		entry[field] = value
	}
 if marketStackEntryRestricted(entry) || storageCount(entry)>1_000_000_000-count {return ErrLoadoutInvalid}
	entry["item_count"] = storageCount(entry) + count
	storage[key] = entry
	updated, err := json.Marshal(storage)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE player_profiles SET balance=balance-$2,inventory=$3,updated_at=now() WHERE user_id=$1`, userID, totalCost, updated); err != nil {
		return err
	}
 if command!="" { if _,err:=tx.ExecContext(ctx,`INSERT INTO storage_purchase_operations(user_id,command_id,item_key,quantity,total_cost) VALUES($1,$2,$3,$4,$5)`,userID,command,key,count,totalCost);err!=nil{return err} }
	return tx.Commit()
}
