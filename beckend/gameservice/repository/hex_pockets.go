package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const HexBasePockets = 6
const HexMaxPockets = 12

var ErrHexPockets = errors.New("HEX loadout exceeds pocket capacity")

func pocketCapacityTx(ctx context.Context, q storageQuerier, user, character int) (int, error) {
	// The template owns the bonus. Set identity and other equipped slots do not
	// participate; mixed green/blue chest/pants stack immediately.
	var bonus int
	err := q.QueryRowContext(ctx, `SELECT COALESCE(sum(`+equipmentPocketBonusSQL+`),0)
 FROM character_equipment ce
 JOIN item_instances ii ON ii.id=ce.item_instance_id
 JOIN item_templates it ON it.id=ii.template_id
 JOIN player_characters pc ON pc.id=ce.character_id
 JOIN player_profiles pp ON pp.user_id=pc.user_id
 WHERE pc.user_id=$1 AND ce.character_id=CASE WHEN $2::int=0 THEN pp.selected_character_id ELSE $2::int END
 AND ce.slot=it.slot AND ce.slot IN ('chest','pants')
 AND (it.class_restriction IS NULL OR it.class_restriction=pc.hero_class_id)
 AND ii.owner_user_id=$1 AND ii.current_character_id=pc.id AND ii.status='equipped'
 AND (ii.durability IS NULL OR ii.durability>0)`, user, character).Scan(&bonus)
	capacity := HexBasePockets + bonus
	if capacity > HexMaxPockets {
		capacity = HexMaxPockets
	}
	return capacity, err
}

func pocketStackTx(ctx context.Context, q storageQuerier, key string) (int, error) {
	if strings.HasPrefix(key, "blueprint_") {
		return 1, nil
	}
	if strings.HasPrefix(key, "scroll_") {
		return 2, nil
	}
	id, err := strconv.Atoi(strings.TrimPrefix(key, "resource_"))
	if err != nil {
		return 0, ErrLoadoutInvalid
	}
	var code string
	if err = q.QueryRowContext(ctx, `SELECT type FROM resources WHERE id=$1 AND resource_kind='resource'`, id).Scan(&code); err != nil {
		return 0, err
	}
	if r, ok := craftRecipe(code); ok && r.PocketStack > 0 {
		return r.PocketStack, nil
	}
	if code == "water" {
		return 5, nil
	}
	if code == "food" {
		return 2, nil
	}
	return 1, nil
}
func pocketUsageTx(ctx context.Context, q storageQuerier, selected map[string]int) (int, error) {
	used := 0
	for key, count := range selected {
		stack, err := pocketStackTx(ctx, q, key)
		if err != nil {
			return 0, err
		}
		if stack <= 0 || count < 0 {
			return 0, ErrLoadoutInvalid
		}
		used += (count + stack - 1) / stack
	}
	return used, nil
}
func validatePocketsTx(ctx context.Context, tx *sql.Tx, user, character int, selected map[string]int) error {
	// Equipment mutations lock the same character row. Keep capacity stable
	// through the actual loadout transfer, including equipment changes in another tab.
	var lockedCharacter int
	if err := tx.QueryRowContext(ctx, `SELECT pc.id FROM player_characters pc JOIN player_profiles pp ON pp.user_id=pc.user_id WHERE pc.user_id=$1 AND pc.id=CASE WHEN $2::int=0 THEN pp.selected_character_id ELSE $2::int END FOR UPDATE OF pc`, user, character).Scan(&lockedCharacter); err != nil {
		return err
	}
	capacity, err := pocketCapacityTx(ctx, tx, user, lockedCharacter)
	if err != nil {
		return err
	}
	used, err := pocketUsageTx(ctx, tx, selected)
	if err != nil {
		return err
	}
	if used > capacity {
		return fmt.Errorf("%w: %d / %d", ErrHexPockets, used, capacity)
	}
	return nil
}
func enrichStoragePockets(ctx context.Context, q storageQuerier, user int, s *StorageState) error {
	var err error
	s.PocketCapacity, err = pocketCapacityTx(ctx, q, user, 0)
	if err != nil {
		return err
	}
	s.PocketStacks = map[string]int{}
	for _, key := range s.Eligible {
		stack, e := pocketStackTx(ctx, q, key)
		if e != nil {
			return e
		}
		s.PocketStacks[key] = stack
	}
	s.PocketsUsed, err = pocketUsageTx(ctx, q, s.Loadout)
	return err
}

// Match inventory has counts, not individually addressable pocket positions.
// Preserve everything and suspend consumable use while over capacity. This also
// covers legacy oversized inventories and equipment changes during a match.
func matchPocketStateTx(ctx context.Context, q storageQuerier, instance string, user int) (int, int, int, error) {
	var character int
	var raw []byte
	if err := q.QueryRowContext(ctx, `SELECT character_id,inventory FROM match_players WHERE instance_id=$1 AND user_id=$2`, instance, user).Scan(&character, &raw); err != nil {
		return 0, 0, 0, err
	}
	inventory, err := parseStorage(raw)
	if err != nil {
		return 0, 0, 0, err
	}
	selected := map[string]int{}
	for key, entry := range inventory {
		if allowedHexLoadoutKeyTx(ctx, q, key) {
			selected[key] = storageCount(entry)
		}
	}
	capacity, err := pocketCapacityTx(ctx, q, user, character)
	if err != nil {
		return 0, 0, 0, err
	}
	used, err := pocketUsageTx(ctx, q, selected)
	return character, capacity, used, err
}

func validateMatchItemPocketsTx(ctx context.Context, tx *sql.Tx, instance string, user int) error {
	// Hold the same character lock as equipment mutations, then reread capacity.
	var character int
	if err := tx.QueryRowContext(ctx, `SELECT pc.id FROM match_players p JOIN player_characters pc ON pc.id=p.character_id AND pc.user_id=p.user_id WHERE p.instance_id=$1 AND p.user_id=$2 FOR UPDATE OF pc`, instance, user).Scan(&character); err != nil {
		return err
	}
	_, capacity, used, err := matchPocketStateTx(ctx, tx, instance, user)
	if err != nil {
		return err
	}
	if used > capacity {
		return fmt.Errorf("%w: %d / %d; restore pocket capacity before using consumables; items remain in inventory", ErrHexPockets, used, capacity)
	}
	return nil
}
