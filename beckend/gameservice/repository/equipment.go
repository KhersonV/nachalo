package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"gameservice/internal/game/balance"
	"sort"
	"strings"
)

var (
	ErrEquipmentCharacterNotFound  = errors.New("equipment character not found")
	ErrEquipmentCharacterNotOwned  = errors.New("equipment character not owned")
	ErrEquipmentItemNotFound       = errors.New("equipment item not found")
	ErrEquipmentItemNotOwned       = errors.New("equipment item not owned")
	ErrEquipmentInvalidSlot        = errors.New("invalid equipment slot")
	ErrEquipmentItemUnavailable    = errors.New("equipment item unavailable")
	ErrEquipmentItemNotInInventory = errors.New("equipment item is not in inventory")
	ErrEquipmentItemLocked         = errors.New("equipment item locked")
	ErrEquipmentItemListed         = errors.New("equipment item listed")
	ErrEquipmentItemTradeLocked    = errors.New("equipment item trade locked")
	ErrEquipmentItemDeleted        = errors.New("equipment item deleted")
	ErrEquipmentItemEquipped       = errors.New("equipment item is equipped")
	ErrEquipmentSetNotFound        = errors.New("equipment set not found")
	ErrEquipmentClassRestricted    = errors.New("equipment class restricted")
	ErrEquipmentLevelTooLow        = errors.New("equipment level too low")
	ErrEquipmentSlotMismatch       = errors.New("equipment slot mismatch")
	ErrEquipmentOffHandBlocked     = errors.New("equipment off hand blocked by two-handed weapon")
	ErrEquipmentInvalidTemplate    = errors.New("equipment template invalid")
)

var validEquipmentSlots = map[string]struct{}{
	"main_hand": {},
	"off_hand":  {},
	"helmet":    {},
	"chest":     {},
	"pants":     {},
	"boots":     {},
	"gloves":    {},
	"ring":      {},
	"amulet":    {},
}

var validEquipmentSources = map[string]struct{}{
	"drop":    {},
	"shop":    {},
	"quest":   {},
	"craft":   {},
	"admin":   {},
	"trade":   {},
	"unknown": {},
}

type EquipmentBonuses struct {
	PocketBonus int `json:"pocketBonus,omitempty"`
	Attack      int `json:"attack"`
	Defense     int `json:"defense"`
	Mobility    int `json:"mobility"`
	Agility     int `json:"agility"`
	MaxHealth   int `json:"maxHealth"`
	MaxEnergy   int `json:"-"`
	SightRange  int `json:"sightRange"`
	AttackRange int `json:"attackRange"`
}

type EquipmentStats struct {
	MaxAP        int  `json:"maxAP"`
	MaxMP        int  `json:"maxMP"`
	MaxReactions int  `json:"maxReactions"`
	MaxHealth    int  `json:"maxHealth"`
	MaxEnergy    int  `json:"-"`
	Attack       int  `json:"attack"`
	Defense      int  `json:"defense"`
	Mobility     int  `json:"mobility"`
	Agility      int  `json:"agility"`
	SightRange   int  `json:"sightRange"`
	IsRanged     bool `json:"isRanged"`
	AttackRange  int  `json:"attackRange"`
}

type EquipmentItem struct {
	Description         string                    `json:"description,omitempty"`
	CombatBonuses       balance.CombatItemBonuses `json:"combatBonuses,omitempty"`
	JewelrySet          *JewelryPair              `json:"jewelrySet,omitempty"`
	InstanceID          string                    `json:"instanceId"`
	TemplateID          int64                     `json:"templateId"`
	Code                string                    `json:"code"`
	Name                string                    `json:"name"`
	SetID               *int64                    `json:"-"`
	SetCode             string                    `json:"setCode,omitempty"`
	SetName             string                    `json:"setName,omitempty"`
	Slot                string                    `json:"slot"`
	ItemType            string                    `json:"itemType"`
	Handedness          string                    `json:"handedness"`
	Rarity              string                    `json:"rarity"`
	ClassRestriction    string                    `json:"classRestriction,omitempty"`
	LevelRequirement    int                       `json:"levelRequirement"`
	ImageURL            string                    `json:"imageUrl"`
	Bonuses             EquipmentBonuses          `json:"bonuses"`
	CanSell             bool                      `json:"canSell"`
	SellBlockReason     string                    `json:"sellBlockReason,omitempty"`
	SellPrice           int                       `json:"sellPrice"`
	IsTwoHanded         bool                      `json:"isTwoHanded,omitempty"`
	Status              string                    `json:"status"`
	Version             int                       `json:"version"`
	EquippedCharacterID *int                      `json:"equippedCharacterId,omitempty"`
}

type ActiveEquipmentSetBonus struct {
	SetCode        string           `json:"setCode"`
	SetName        string           `json:"setName"`
	Pieces         int              `json:"pieces"`
	PiecesRequired int              `json:"piecesRequired"`
	Description    string           `json:"description"`
	Bonuses        EquipmentBonuses `json:"bonuses"`
}

type CharacterEffectiveStats struct {
	RawBaseStats      *EquipmentStats           `json:"-"`
	RawEffectiveStats *EquipmentStats           `json:"-"`
	CombatProfile     balance.CombatProfile     `json:"combatProfile"`
	CharacterID       int                       `json:"characterId"`
	HeroClassID       string                    `json:"heroClassId"`
	BaseStats         EquipmentStats            `json:"baseStats"`
	EffectiveStats    EquipmentStats            `json:"effectiveStats"`
	Equipped          map[string]*EquipmentItem `json:"equipped"`
	ActiveSetBonuses  []ActiveEquipmentSetBonus `json:"activeSetBonuses"`
}

type EquipmentCharacterSummary struct {
	CharacterID int    `json:"characterId"`
	HeroClassID string `json:"heroClassId"`
	Name        string `json:"name"`
	Level       int    `json:"level"`
}

type equipmentItemLock struct {
	EquipmentItem
	OwnerUserID int
	IsLocked    bool
}

type equippedSlotLock struct {
	Slot           string
	ItemInstanceID string
	OwnerUserID    int
	Handedness     string
}

type equipmentDispositionItemLock struct {
	InstanceID         string
	OwnerUserID        int
	Status             string
	CurrentCharacterID sql.NullInt64
	IsLocked           bool
	HasEquipmentRow    bool
	SaleRestricted     bool
	SellPrice          int
	Handedness         string
	Rarity             string
	Slot               string
	ItemType           string
}

func normalizeEquipmentSlot(slot string) string {
	return strings.ToLower(strings.TrimSpace(slot))
}

func isValidEquipmentSlot(slot string) bool {
	_, ok := validEquipmentSlots[normalizeEquipmentSlot(slot)]
	return ok
}

func normalizeEquipmentSource(source string) string {
	source = strings.ToLower(strings.TrimSpace(source))
	if _, ok := validEquipmentSources[source]; ok {
		return source
	}
	return "unknown"
}

func normalizeEquipmentCode(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	code = strings.ReplaceAll(code, " ", "_")
	code = strings.ReplaceAll(code, "-", "_")
	return code
}

func equipmentStatsFromCharacter(character *PlayerCharacter) EquipmentStats {
	reactions := 1
	if character.HeroClassID == "guardian" {
		reactions = 2
	}
	return EquipmentStats{MaxAP: 6, MaxMP: character.Mobility * 2, MaxReactions: reactions,
		MaxHealth:   character.MaxHealth,
		MaxEnergy:   character.MaxEnergy,
		Attack:      character.Attack,
		Defense:     character.Defense,
		Mobility:    character.Mobility,
		Agility:     character.Agility,
		SightRange:  character.SightRange,
		IsRanged:    character.IsRanged,
		AttackRange: character.AttackRange,
	}
}

func addEquipmentBonuses(stats *EquipmentStats, bonuses EquipmentBonuses) {
	// PocketBonus is evaluated from equipped chest/pants by pocketCapacityTx,
	// never through combat stats or item_set_bonuses.
	stats.Attack += bonuses.Attack
	stats.Defense += bonuses.Defense
	stats.Mobility += bonuses.Mobility
	stats.Agility += bonuses.Agility
	stats.MaxHealth += bonuses.MaxHealth
	// Energy bonuses are retired; catalog entries are explicitly converted in Stage 9.
	stats.SightRange += bonuses.SightRange
	stats.AttackRange += bonuses.AttackRange
}

func scanEquipmentItem(scan func(dest ...interface{}) error) (*EquipmentItem, error) {
	var item EquipmentItem
	var setID sql.NullInt64
	var setCode sql.NullString
	var setName sql.NullString
	var classRestriction sql.NullString
	var equippedCharacterID sql.NullInt64
	err := scan(
		&item.InstanceID,
		&item.TemplateID,
		&item.Code,
		&item.Name,
		&setID,
		&setCode,
		&setName,
		&item.Slot,
		&item.ItemType,
		&item.Handedness,
		&item.Rarity,
		&classRestriction,
		&item.LevelRequirement,
		&item.ImageURL,
		&item.Bonuses.Attack,
		&item.Bonuses.Defense,
		&item.Bonuses.Mobility,
		&item.Bonuses.Agility,
		&item.Bonuses.MaxHealth,
		&item.Bonuses.MaxEnergy,
		&item.Bonuses.SightRange,
		&item.Bonuses.AttackRange,
		&item.Bonuses.PocketBonus,
		&item.SellPrice,
		&item.Status,
		&item.Version,
		&equippedCharacterID,
		&item.SellBlockReason,
	)
	if err != nil {
		return nil, err
	}
	if setID.Valid {
		v := setID.Int64
		item.SetID = &v
	}
	if setCode.Valid {
		item.SetCode = setCode.String
	}
	if setName.Valid {
		item.SetName = setName.String
	}
	if classRestriction.Valid {
		item.ClassRestriction = classRestriction.String
	}
	if equippedCharacterID.Valid {
		v := int(equippedCharacterID.Int64)
		item.EquippedCharacterID = &v
	}

	if catalog, _, ok := equipmentCatalogItem(item.Code); ok {
		item.CombatBonuses = catalog.CombatBonuses
		item.Description = catalog.Description
	}
	if _, set, ok := jewelryItem(item.Code); ok {
		item.JewelrySet = &JewelryPair{Name: set.Name, Bonus: set.PairBonus, Required: 2}
	}
	// Compute canonical sell price and two-handed flag based on template metadata
	item.IsTwoHanded = item.Handedness == "two_hand"

	item.CanSell = item.SellBlockReason == ""
	return &item, nil
}

func equipmentItemSelectClause() string {
	return equipmentInstanceColumnsSQL(`
		ii.id::text,
		ii.template_id,
		it.code,
		it.name,
		it.set_id,
		s.code,
		s.name,
		it.slot,
		it.item_type,
		it.handedness,
		it.rarity,
		it.class_restriction,
		it.level_requirement,
		it.image_url,
		it.attack_bonus,
		it.defense_bonus,
		it.mobility_bonus,
		it.agility_bonus,
		it.max_health_bonus,
		it.max_energy_bonus,
		it.sight_range_bonus,
		it.attack_range_bonus,
        ` + equipmentPocketBonusSQL + `,
		it.sell_price,
		ii.status,
		ii.version,
		ii.current_character_id,
        CASE
         WHEN ` + commonEquipmentSQL("it") + ` THEN 'Common equipment has been retired'
         WHEN ii.status <> 'inventory' OR ii.current_character_id IS NOT NULL OR EXISTS(SELECT 1 FROM character_equipment ce WHERE ce.item_instance_id=ii.id) THEN 'Unequip this item or release its trade reservation first'
         WHEN ii.is_locked THEN 'Item is locked'
         WHEN ii.is_soulbound OR ii.bound_to_user_id IS NOT NULL OR ii.bound_to_character_id IS NOT NULL THEN 'Bound items cannot be sold'
         WHEN NOT it.tradable THEN 'This item cannot be sold'
         WHEN ii.match_instance_id IS NOT NULL THEN 'Extract this item from the match first'
         ELSE '' END
	`)
}

func GetEquipmentInventoryForUser(userID int) ([]EquipmentItem, error) {
	rows, err := DB.Query(`
		SELECT `+equipmentItemSelectClause()+`
		FROM item_instances ii
		JOIN item_templates it ON it.id = ii.template_id
		LEFT JOIN item_sets s ON s.id = it.set_id
		WHERE ii.owner_user_id = $1
		  AND ii.status = 'inventory' AND NOT (`+commonEquipmentSQL("it")+`)
		ORDER BY it.slot, it.rarity, it.name, ii.acquired_at, ii.id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("GetEquipmentInventoryForUser query: %w", err)
	}
	defer rows.Close()

	items := []EquipmentItem{}
	for rows.Next() {
		item, err := scanEquipmentItem(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("GetEquipmentInventoryForUser scan: %w", err)
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetEquipmentInventoryForUser rows: %w", err)
	}
	return items, nil
}

func GetAllEquipmentForUser(userID int) ([]EquipmentItem, error) {
	rows, err := DB.Query(`
		SELECT `+equipmentItemSelectClause()+`
		FROM item_instances ii
		JOIN item_templates it ON it.id = ii.template_id
		LEFT JOIN item_sets s ON s.id = it.set_id
		WHERE ii.owner_user_id = $1
		  AND ii.status <> 'deleted' AND NOT (`+commonEquipmentSQL("it")+`)
		ORDER BY it.class_restriction, s.name, it.slot, it.name, ii.acquired_at, ii.id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("GetAllEquipmentForUser query: %w", err)
	}
	defer rows.Close()

	items := []EquipmentItem{}
	for rows.Next() {
		item, err := scanEquipmentItem(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("GetAllEquipmentForUser scan: %w", err)
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetAllEquipmentForUser rows: %w", err)
	}
	return items, nil
}

func GetCharacterEquipment(characterID int) (map[string]*EquipmentItem, error) {
	rows, err := DB.Query(`
		SELECT `+equipmentItemSelectClause()+`
		FROM character_equipment ce
		JOIN item_instances ii ON ii.id = ce.item_instance_id
		JOIN item_templates it ON it.id = ii.template_id
		LEFT JOIN item_sets s ON s.id = it.set_id
		WHERE ce.character_id = $1 AND NOT (`+commonEquipmentSQL("it")+`)
		ORDER BY ce.slot
	`, characterID)
	if err != nil {
		return nil, fmt.Errorf("GetCharacterEquipment query: %w", err)
	}
	defer rows.Close()

	equipped := map[string]*EquipmentItem{}
	for rows.Next() {
		item, err := scanEquipmentItem(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("GetCharacterEquipment scan: %w", err)
		}
		itemCopy := *item
		equipped[itemCopy.Slot] = &itemCopy
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetCharacterEquipment rows: %w", err)
	}
	return equipped, nil
}

func ListPlayerEquipmentCharacters(userID int) ([]EquipmentCharacterSummary, error) {
	rows, err := DB.Query(`
		SELECT id, hero_class_id, level
		FROM player_characters
		WHERE user_id = $1
		ORDER BY id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("ListPlayerEquipmentCharacters query: %w", err)
	}
	defer rows.Close()

	characters := []EquipmentCharacterSummary{}
	for rows.Next() {
		var character EquipmentCharacterSummary
		if err := rows.Scan(&character.CharacterID, &character.HeroClassID, &character.Level); err != nil {
			return nil, fmt.Errorf("ListPlayerEquipmentCharacters scan: %w", err)
		}
		character.HeroClassID = NormalizeHeroClassID(character.HeroClassID)
		character.Name = ResolveHeroClassMeta(character.HeroClassID).DisplayName
		characters = append(characters, character)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListPlayerEquipmentCharacters rows: %w", err)
	}
	return characters, nil
}

func selectPlayerCharacterByIDAny(characterID int) (*PlayerCharacter, error) {
	return scanPlayerCharacter(DB.QueryRow(`
		SELECT
			id,
			user_id,
			hero_class_id,
			COALESCE(image, ''),
			level,
			exp,
			max_exp,
			max_energy,
			max_health,
			attack,
			defense,
			mobility,
			agility,
			sight_range,
			is_ranged,
			attack_range,
			COALESCE(source, '')
		FROM player_characters
		WHERE id = $1
	`, characterID).Scan)
}

func selectPlayerCharacterByIDForUpdateTx(tx *sql.Tx, userID int, characterID int) (*PlayerCharacter, error) {
	character, err := scanPlayerCharacter(tx.QueryRow(`
		SELECT
			id,
			user_id,
			hero_class_id,
			COALESCE(image, ''),
			level,
			exp,
			max_exp,
			max_energy,
			max_health,
			attack,
			defense,
			mobility,
			agility,
			sight_range,
			is_ranged,
			attack_range,
			COALESCE(source, '')
		FROM player_characters
		WHERE id = $1
		FOR UPDATE
	`, characterID).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEquipmentCharacterNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock character scan: %w", err)
	}
	if character.UserID != userID {
		return nil, ErrEquipmentCharacterNotOwned
	}
	return character, nil
}

func lockItemInstanceTx(tx *sql.Tx, itemInstanceID string) (*equipmentItemLock, error) {
	itemInstanceID = strings.TrimSpace(itemInstanceID)
	if itemInstanceID == "" {
		return nil, ErrEquipmentItemNotFound
	}

	var row equipmentItemLock
	item, err := scanEquipmentItem(func(dest ...interface{}) error {
		allDest := append(dest, &row.OwnerUserID, &row.IsLocked)
		return tx.QueryRow(`
			SELECT `+equipmentItemSelectClause()+`,
				ii.owner_user_id,
				ii.is_locked
			FROM item_instances ii
			JOIN item_templates it ON it.id = ii.template_id
			LEFT JOIN item_sets s ON s.id = it.set_id
			WHERE ii.id = $1::uuid
			FOR UPDATE OF ii
		`, itemInstanceID).Scan(allDest...)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEquipmentItemNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock item instance scan item/template: %w", err)
	}
	row.EquipmentItem = *item
	return &row, nil
}

func validateTemplateShape(item EquipmentItem) error {
	switch item.ItemType {
	case "sword":
		if item.Slot != "main_hand" || item.Handedness != "one_hand" {
			return ErrEquipmentInvalidTemplate
		}
	case "shield":
		if item.Slot != "off_hand" || item.Handedness != "one_hand" {
			return ErrEquipmentInvalidTemplate
		}
	case "staff", "bow", "axe":
		if item.Slot != "main_hand" || item.Handedness != "two_hand" {
			return ErrEquipmentInvalidTemplate
		}
	case "helmet", "chest", "pants", "boots", "gloves", "ring", "amulet":
		if item.Slot != item.ItemType || item.Handedness != "none" {
			return ErrEquipmentInvalidTemplate
		}
	default:
		return ErrEquipmentInvalidTemplate
	}
	return nil
}

func validateItemForEquip(character *PlayerCharacter, item *equipmentItemLock) error {
	if isCommonEquipment(item.Rarity, item.Slot) {
		return ErrEquipmentInvalidTemplate
	}
	if item.OwnerUserID != character.UserID {
		return ErrEquipmentItemNotOwned
	}
	switch item.Status {
	case "deleted":
		return ErrEquipmentItemDeleted
	case "listed":
		return ErrEquipmentItemListed
	case "trade_locked":
		return ErrEquipmentItemTradeLocked
	}
	if item.IsLocked {
		return ErrEquipmentItemLocked
	}
	if item.Status != "inventory" {
		return ErrEquipmentItemNotInInventory
	}
	if item.ClassRestriction != "" && item.ClassRestriction != character.HeroClassID {
		return ErrEquipmentClassRestricted
	}
	if item.LevelRequirement > character.Level {
		return ErrEquipmentLevelTooLow
	}
	if err := validateTemplateShape(item.EquipmentItem); err != nil {
		return err
	}
	if normalizeEquipmentSlot(item.Slot) != item.Slot || !isValidEquipmentSlot(item.Slot) {
		return ErrEquipmentSlotMismatch
	}
	return nil
}

func lockEquippedSlotsTx(tx *sql.Tx, characterID int, slotA string, slotB string) ([]equippedSlotLock, error) {
	rows, err := tx.Query(`
		SELECT
			ce.slot,
			ii.id::text,
			ii.owner_user_id,
			it.handedness
		FROM character_equipment ce
		JOIN item_instances ii ON ii.id = ce.item_instance_id
		JOIN item_templates it ON it.id = ii.template_id
		WHERE ce.character_id = $1
		  AND (ce.slot = $2 OR ce.slot = $3)
		FOR UPDATE
	`, characterID, slotA, slotB)
	if err != nil {
		return nil, fmt.Errorf("lockEquippedSlotsTx query: %w", err)
	}
	defer rows.Close()

	var locked []equippedSlotLock
	for rows.Next() {
		var item equippedSlotLock
		if err := rows.Scan(&item.Slot, &item.ItemInstanceID, &item.OwnerUserID, &item.Handedness); err != nil {
			return nil, fmt.Errorf("lockEquippedSlotsTx scan: %w", err)
		}
		locked = append(locked, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lockEquippedSlotsTx rows: %w", err)
	}
	return locked, nil
}

func insertItemInstanceEventTx(tx *sql.Tx, itemInstanceID string, eventType string, fromUserID *int, toUserID *int, fromCharacterID *int, toCharacterID *int, slot string) error {
	var metadata string
	if slot == "" {
		metadata = `{}`
	} else {
		metadata = fmt.Sprintf(`{"slot":%q}`, slot)
	}
	fromUser := nullableInt64FromIntPtr(fromUserID)
	toUser := nullableInt64FromIntPtr(toUserID)
	fromCharacter := nullableInt64FromIntPtr(fromCharacterID)
	toCharacter := nullableInt64FromIntPtr(toCharacterID)
	_, err := tx.Exec(`
		INSERT INTO item_instance_events (
			item_instance_id,
			event_type,
			from_user_id,
			to_user_id,
			from_character_id,
			to_character_id,
			metadata
		)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7::jsonb)
	`, itemInstanceID, eventType, fromUser, toUser, fromCharacter, toCharacter, metadata)
	if err != nil {
		return fmt.Errorf("insert item_instance_events %s: %w", eventType, err)
	}
	return nil
}

func nullableInt64FromIntPtr(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

func unequipLockedSlotsTx(tx *sql.Tx, characterID int, slots []string, existing []equippedSlotLock) error {
	if len(existing) == 0 {
		return nil
	}
	for _, equipped := range existing {
		ownerID := equipped.OwnerUserID
		fromCharacterID := characterID
		if _, err := tx.Exec(`
			UPDATE item_instances
			SET status = 'inventory',
				current_character_id = NULL,
				updated_at = now(),
				version = version + 1
			WHERE id = $1::uuid
		`, equipped.ItemInstanceID); err != nil {
			return fmt.Errorf("unequipLockedSlotsTx update item %s: %w", equipped.ItemInstanceID, err)
		}
		if err := insertItemInstanceEventTx(tx, equipped.ItemInstanceID, "unequipped", &ownerID, &ownerID, &fromCharacterID, nil, equipped.Slot); err != nil {
			return fmt.Errorf("insert item_instance_events unequipped: %w", err)
		}
	}

	if len(slots) == 1 {
		if _, err := tx.Exec(`
			DELETE FROM character_equipment
			WHERE character_id = $1
			  AND slot = $2
		`, characterID, slots[0]); err != nil {
			return fmt.Errorf("unequipLockedSlotsTx delete one slot: %w", err)
		}
		return nil
	}
	if _, err := tx.Exec(`
		DELETE FROM character_equipment
		WHERE character_id = $1
		  AND (slot = $2 OR slot = $3)
	`, characterID, slots[0], slots[1]); err != nil {
		return fmt.Errorf("unequipLockedSlotsTx delete slots: %w", err)
	}
	return nil
}

func EquipItemToCharacter(userID int, characterID int, itemInstanceID string) error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("EquipItemToCharacter begin tx: %w", err)
	}
	defer tx.Rollback()

	character, err := selectPlayerCharacterByIDForUpdateTx(tx, userID, characterID)
	if err != nil {
		return fmt.Errorf("lock character: %w", err)
	}

	item, err := lockItemInstanceTx(tx, itemInstanceID)
	if err != nil {
		return fmt.Errorf("lock item instance: %w", err)
	}
	if err := validateItemForEquip(character, item); err != nil {
		return fmt.Errorf("validate item for equip: %w", err)
	}

	targetSlot := item.Slot
	slotsToUnequip := []string{targetSlot}
	if item.Handedness == "two_hand" {
		slotsToUnequip = []string{"main_hand", "off_hand"}
	} else if targetSlot == "off_hand" {
		mainHand, err := lockEquippedSlotsTx(tx, characterID, "main_hand", "main_hand")
		if err != nil {
			return fmt.Errorf("select current main_hand: %w", err)
		}
		if len(mainHand) > 0 && mainHand[0].Handedness == "two_hand" {
			return ErrEquipmentOffHandBlocked
		}
	}

	slotA := slotsToUnequip[0]
	slotB := slotA
	if len(slotsToUnequip) > 1 {
		slotB = slotsToUnequip[1]
	}
	existing, err := lockEquippedSlotsTx(tx, characterID, slotA, slotB)
	if err != nil {
		return fmt.Errorf("select currently equipped slot: %w", err)
	}
	if err := unequipLockedSlotsTx(tx, characterID, slotsToUnequip, existing); err != nil {
		return fmt.Errorf("unequip replaced items: %w", err)
	}

	if _, err := tx.Exec(`
		INSERT INTO character_equipment (character_id, item_instance_id, slot)
		VALUES ($1, $2::uuid, $3)
	`, characterID, item.InstanceID, targetSlot); err != nil {
		return fmt.Errorf("insert character_equipment: %w", err)
	}

	if _, err := tx.Exec(`
		UPDATE item_instances
		SET status = 'equipped',
			current_character_id = $1,
			updated_at = now(),
			version = version + 1
		WHERE id = $2::uuid
	`, characterID, item.InstanceID); err != nil {
		return fmt.Errorf("update item_instances equipped: %w", err)
	}

	toUserID := userID
	toCharacterID := characterID
	if err := insertItemInstanceEventTx(tx, item.InstanceID, "equipped", &toUserID, &toUserID, nil, &toCharacterID, targetSlot); err != nil {
		return fmt.Errorf("insert item_instance_events equipped: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit equipment equip: %w", err)
	}
	return nil
}

func UnequipItemFromCharacter(userID int, characterID int, slot string) error {
	slot = normalizeEquipmentSlot(slot)
	if !isValidEquipmentSlot(slot) {
		return ErrEquipmentInvalidSlot
	}

	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("UnequipItemFromCharacter begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := selectPlayerCharacterByIDForUpdateTx(tx, userID, characterID); err != nil {
		return err
	}

	existing, err := lockEquippedSlotsTx(tx, characterID, slot, slot)
	if err != nil {
		return err
	}
	if len(existing) == 0 {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("UnequipItemFromCharacter noop commit: %w", err)
		}
		return nil
	}

	if err := unequipLockedSlotsTx(tx, characterID, []string{slot}, existing); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("UnequipItemFromCharacter commit: %w", err)
	}
	return nil
}

func lockDispositionItemTx(tx *sql.Tx, itemInstanceID string) (*equipmentDispositionItemLock, error) {
	itemInstanceID = strings.TrimSpace(itemInstanceID)
	if itemInstanceID == "" {
		return nil, ErrEquipmentItemNotFound
	}

	var item equipmentDispositionItemLock
	err := tx.QueryRow(`
		SELECT
			ii.id::text,
			ii.owner_user_id,
			ii.status,
			ii.current_character_id,
			ii.is_locked,
			EXISTS (
				SELECT 1
				FROM character_equipment ce
				WHERE ce.item_instance_id = ii.id
			),
			it.sell_price,
			it.handedness,
			it.rarity,
			it.slot,
			it.item_type,
            (ii.is_soulbound OR ii.bound_to_user_id IS NOT NULL OR ii.bound_to_character_id IS NOT NULL OR NOT it.tradable OR ii.match_instance_id IS NOT NULL)
		FROM item_instances ii
		JOIN item_templates it ON it.id = ii.template_id
		WHERE ii.id = $1::uuid
		FOR UPDATE OF ii
		`, itemInstanceID).Scan(
		&item.InstanceID,
		&item.OwnerUserID,
		&item.Status,
		&item.CurrentCharacterID,
		&item.IsLocked,
		&item.HasEquipmentRow,
		&item.SellPrice,
		&item.Handedness,
		&item.Rarity,
		&item.Slot,
		&item.ItemType,
		&item.SaleRestricted,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEquipmentItemNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lockDispositionItemTx scan item: %w", err)
	}

	return &item, nil
}

func validateItemForDisposition(userID int, item *equipmentDispositionItemLock) error {
	if item.OwnerUserID != userID {
		return ErrEquipmentItemNotOwned
	}
	if item.IsLocked {
		return ErrEquipmentItemLocked
	}
	switch item.Status {
	case "inventory":
		if item.CurrentCharacterID.Valid || item.HasEquipmentRow {
			return ErrEquipmentItemEquipped
		}
		return nil
	case "equipped":
		return ErrEquipmentItemEquipped
	case "deleted":
		return ErrEquipmentItemDeleted
	case "listed":
		return ErrEquipmentItemListed
	case "trade_locked":
		return ErrEquipmentItemTradeLocked
	default:
		return ErrEquipmentItemUnavailable
	}
}

func SellEquipmentItem(userID int, itemInstanceID string) (int, error) {
	balance, _, err := SellEquipmentItemWithPrice(userID, itemInstanceID)
	return balance, err
}

// Return the exact price credited inside the sale transaction for the response.
func SellEquipmentItemWithPrice(userID int, itemInstanceID string) (int, int, error) {
	tx, err := DB.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("SellEquipmentItem begin tx: %w", err)
	}
	defer tx.Rollback()

	item, err := lockDispositionItemTx(tx, itemInstanceID)
	if err != nil {
		return 0, 0, err
	}
	if err := validateItemForDisposition(userID, item); err != nil {
		return 0, 0, err
	}

	if item.SaleRestricted || isCommonEquipment(item.Rarity, item.Slot) {
		return 0, 0, ErrEquipmentItemUnavailable
	}
	if _, err := tx.Exec(`
		UPDATE item_instances
		SET status = 'deleted',
			current_character_id = NULL,
			updated_at = now(),
			version = version + 1
		WHERE id = $1::uuid
	`, item.InstanceID); err != nil {
		return 0, 0, fmt.Errorf("SellEquipmentItem update item: %w", err)
	}

	fromUserID := userID
	if err := insertItemInstanceEventTx(tx, item.InstanceID, "sold", &fromUserID, nil, nil, nil, ""); err != nil {
		return 0, 0, fmt.Errorf("SellEquipmentItem insert sold event: %w", err)
	}

	var balance int
	if err := tx.QueryRow(`
		UPDATE player_profiles
		SET balance = balance + $1,
			updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $2
		RETURNING balance
	`, item.SellPrice, userID).Scan(&balance); err != nil {
		return 0, 0, fmt.Errorf("SellEquipmentItem update balance: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("SellEquipmentItem commit: %w", err)
	}
	return balance, item.SellPrice, nil
}

func DiscardEquipmentItem(userID int, itemInstanceID string) error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("DiscardEquipmentItem begin tx: %w", err)
	}
	defer tx.Rollback()

	item, err := lockDispositionItemTx(tx, itemInstanceID)
	if err != nil {
		return err
	}
	if err := validateItemForDisposition(userID, item); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		UPDATE item_instances
		SET status = 'deleted',
			current_character_id = NULL,
			updated_at = now(),
			version = version + 1
		WHERE id = $1::uuid
	`, item.InstanceID); err != nil {
		return fmt.Errorf("DiscardEquipmentItem update item: %w", err)
	}

	fromUserID := userID
	if err := insertItemInstanceEventTx(tx, item.InstanceID, "deleted", &fromUserID, nil, nil, nil, ""); err != nil {
		return fmt.Errorf("DiscardEquipmentItem insert deleted event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("DiscardEquipmentItem commit: %w", err)
	}
	return nil
}

func GrantItemInstanceToUser(userID int, templateCode string, source string) (*EquipmentItem, error) {
	templateCode = strings.TrimSpace(templateCode)
	if templateCode == "" {
		return nil, ErrEquipmentInvalidTemplate
	}
	source = normalizeEquipmentSource(source)
	if _, _, jewelry := jewelryItem(templateCode); jewelry && source != "craft" {
		return nil, ErrEquipmentInvalidTemplate
	}
	dropSet := ""
	if source == "drop" {
		_, set, ok := equipmentCatalogItem(templateCode)
		if !ok || set.Source != "monster" || set.Rarity != "green" {
			return nil, ErrEquipmentInvalidTemplate
		}
		dropSet = set.Code
	}

	tx, err := DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("GrantItemInstanceToUser begin tx: %w", err)
	}
	defer tx.Rollback()

	var profileExists bool
	if err := tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM player_profiles WHERE user_id = $1)`, userID).Scan(&profileExists); err != nil {
		return nil, fmt.Errorf("GrantItemInstanceToUser check profile: %w", err)
	}
	if !profileExists {
		return nil, ErrPlayerNotFound
	}

	var itemInstanceID string
	if err := tx.QueryRow(`
		INSERT INTO item_instances (template_id, owner_user_id, source)
		SELECT id, $1, $2
		FROM item_templates
		WHERE code = $3 AND NOT (`+commonEquipmentSQL("")+`) AND ($4<>'drop' OR (rarity='green' AND set_id=(SELECT id FROM item_sets WHERE code=$5 AND rarity='green')))
		RETURNING id::text
	`, userID, source, templateCode, source, dropSet).Scan(&itemInstanceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEquipmentInvalidTemplate
		}
		return nil, fmt.Errorf("GrantItemInstanceToUser insert item: %w", err)
	}

	toUserID := userID
	if err := insertItemInstanceEventTx(tx, itemInstanceID, "created", nil, &toUserID, nil, nil, ""); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("GrantItemInstanceToUser commit: %w", err)
	}

	return getEquipmentItemByID(itemInstanceID)
}

func GrantEquipmentSetToUser(userID int, heroClassID string, setCode string, source string) error {
	heroClassID = NormalizeHeroClassID(heroClassID)
	if !IsKnownHeroClassID(heroClassID) {
		return ErrEquipmentClassRestricted
	}
	setCode = normalizeEquipmentCode(setCode)
	if setCode == "" {
		var ok bool
		setCode, ok = defaultEquipmentSetCodeForClass(heroClassID)
		if !ok {
			return ErrEquipmentSetNotFound
		}
	}
	source = normalizeEquipmentSource(source)
	if source == "drop" {
		return ErrEquipmentInvalidTemplate
	} // Random drops grant individual whitelisted items only.

	if strings.HasPrefix(setCode, "jewelry_") && source != "craft" {
		return ErrEquipmentInvalidTemplate
	}
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("GrantEquipmentSetToUser begin tx: %w", err)
	}
	defer tx.Rollback()

	var profileExists bool
	if err := tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM player_profiles WHERE user_id = $1)`, userID).Scan(&profileExists); err != nil {
		return fmt.Errorf("GrantEquipmentSetToUser check profile: %w", err)
	}
	if !profileExists {
		return ErrPlayerNotFound
	}

	var setID int64
	if err := tx.QueryRow(`
		SELECT id
		FROM item_sets
		WHERE class_restriction = $1
		  AND rarity NOT IN ('gray','grey','common')
		  AND (code = $2 OR code = $2 || '_set')
		LIMIT 1
	`, heroClassID, setCode).Scan(&setID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrEquipmentSetNotFound
		}
		return fmt.Errorf("GrantEquipmentSetToUser select set: %w", err)
	}

	rows, err := tx.Query(`
		SELECT id
		FROM item_templates
		WHERE set_id = $1 AND NOT (`+commonEquipmentSQL("")+`)
		  AND class_restriction = $2
		ORDER BY slot, name
	`, setID, heroClassID)
	if err != nil {
		return fmt.Errorf("GrantEquipmentSetToUser select templates: %w", err)
	}
	defer rows.Close()

	var templateIDs []int64
	for rows.Next() {
		var templateID int64
		if err := rows.Scan(&templateID); err != nil {
			return fmt.Errorf("GrantEquipmentSetToUser scan template: %w", err)
		}
		templateIDs = append(templateIDs, templateID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("GrantEquipmentSetToUser template rows: %w", err)
	}
	rows.Close()
	if len(templateIDs) == 0 {
		return ErrEquipmentSetNotFound
	}

	toUserID := userID
	for _, templateID := range templateIDs {
		var existingID string
		err := tx.QueryRow(`
			SELECT id::text
			FROM item_instances
			WHERE owner_user_id = $1
			  AND template_id = $2
			  AND status <> 'deleted'
			ORDER BY acquired_at, id
			LIMIT 1
		`, userID, templateID).Scan(&existingID)
		if err == nil {
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("GrantEquipmentSetToUser check existing item: %w", err)
		}

		var itemInstanceID string
		if err := tx.QueryRow(`
			INSERT INTO item_instances (template_id, owner_user_id, source)
			VALUES ($1, $2, $3)
			RETURNING id::text
		`, templateID, userID, source).Scan(&itemInstanceID); err != nil {
			return fmt.Errorf("GrantEquipmentSetToUser insert item: %w", err)
		}
		if err := insertItemInstanceEventTx(tx, itemInstanceID, "created", nil, &toUserID, nil, nil, ""); err != nil {
			return fmt.Errorf("GrantEquipmentSetToUser insert created event: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("GrantEquipmentSetToUser commit: %w", err)
	}
	return nil
}

// GrantEquipmentDropToUser is the narrow future drop-system entry point.
// TODO(equipment-drops): when a monster dies, roll a drop chance, choose a
// template code, grant it to the killer user with source=drop, and surface the
// created item on the match result screen. Do not wire this into combat until
// the drop table and result presentation are defined.
func GrantEquipmentDropToUser(userID int, templateCode string) (*EquipmentItem, error) {
	return GrantItemInstanceToUser(userID, templateCode, "drop")
}

func getEquipmentItemByID(itemInstanceID string) (*EquipmentItem, error) {
	return scanEquipmentItem(DB.QueryRow(`
		SELECT `+equipmentItemSelectClause()+`
		FROM item_instances ii
		JOIN item_templates it ON it.id = ii.template_id
		LEFT JOIN item_sets s ON s.id = it.set_id
		WHERE ii.id = $1::uuid
	`, itemInstanceID).Scan)
}

func GetCharacterEffectiveStats(characterID int) (*CharacterEffectiveStats, error) {
	character, err := selectPlayerCharacterByIDAny(characterID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEquipmentCharacterNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("GetCharacterEffectiveStats character: %w", err)
	}

	equipped, err := GetCharacterEquipment(characterID)
	if err != nil {
		return nil, err
	}

	baseStats := equipmentStatsFromCharacter(character)
	effectiveStats := baseStats

	setCounts := map[int64]int{}
	legacyCounts := map[int64]int{}
	setCodeByID := map[int64]string{}
	setNameByID := map[int64]string{}
	for _, item := range equipped {
		if item.JewelrySet != nil && item.ClassRestriction != character.HeroClassID {
			continue
		}
		addEquipmentBonuses(&effectiveStats, item.Bonuses)
		if item.SetID != nil {
			setID := *item.SetID
			setCounts[setID]++
			if legacyMainEquipment(item) {
				legacyCounts[setID]++
			}
			setCodeByID[setID] = item.SetCode
			setNameByID[setID] = item.SetName
		}
	}

	activeSetBonuses, err := loadActiveSetBonuses(setCounts, setCodeByID, setNameByID, legacyCounts)
	if err != nil {
		return nil, err
	}
	for _, bonus := range activeSetBonuses {
		addEquipmentBonuses(&effectiveStats, bonus.Bonuses)
	}

	if effectiveStats.MaxHealth < 1 {
		effectiveStats.MaxHealth = 1
	}
	if effectiveStats.Attack < 0 {
		effectiveStats.Attack = 0
	}
	if effectiveStats.Defense < 0 {
		effectiveStats.Defense = 0
	}
	if effectiveStats.Agility < 0 {
		effectiveStats.Agility = 0
	}
	if effectiveStats.Mobility < 0 {
		effectiveStats.Mobility = 0
	}
	effectiveStats.MaxMP = effectiveStats.Mobility * 2
	if effectiveStats.MaxMP > 16 {
		effectiveStats.MaxMP = 16
	}
	result := &CharacterEffectiveStats{
		CombatProfile:    characterCombatProfile(character.HeroClassID, equipped),
		CharacterID:      character.ID,
		HeroClassID:      character.HeroClassID,
		BaseStats:        baseStats,
		EffectiveStats:   effectiveStats,
		Equipped:         equipped,
		ActiveSetBonuses: activeSetBonuses,
	}
	ruleset, err := configuredEquipmentRuleset()
	if err != nil {
		return nil, err
	}
	if ruleset == balance.ExperimentalV34 {
		result.RawBaseStats = &baseStats
		result.RawEffectiveStats = &effectiveStats
		cfg := balance.ExperimentalV34Config()
		result.EffectiveStats, result.CombatProfile = normalizedV34Loadout(result, &cfg)
		base := *result
		base.Equipped = nil
		base.RawEffectiveStats = &baseStats
		result.BaseStats, _ = normalizedV34Loadout(&base, &cfg)
		for i := range result.ActiveSetBonuses {
			if mainEquipmentSet(result.ActiveSetBonuses[i].SetCode) {
				b := &result.ActiveSetBonuses[i].Bonuses
				b.MaxHealth = 0
				b.Attack = 0
				b.Defense = 0
				b.Agility = 0
				result.ActiveSetBonuses[i].Description = fmt.Sprintf("v34: Mobility %+d, Sight %+d, Range %+d", b.Mobility, b.SightRange, b.AttackRange)
			}
		}
	}
	if ruleset == balance.Legacy {
		projected := legacySnapshotStats(result)
		if projected != effectiveStats {
			result.RawBaseStats = &baseStats
			result.RawEffectiveStats = &effectiveStats
			result.EffectiveStats = projected
			result.BaseStats.MaxHealth += projected.MaxHealth - effectiveStats.MaxHealth
			result.BaseStats.Attack += projected.Attack - effectiveStats.Attack
			result.BaseStats.Defense += projected.Defense - effectiveStats.Defense
		}
	}
	return result, nil
}

func GetCharacterEquipmentBonuses(characterID int) (EquipmentBonuses, error) {
	stats, err := GetCharacterEffectiveStats(characterID)
	if err != nil {
		return EquipmentBonuses{}, err
	}
	return EquipmentBonuses{
		Attack:      stats.EffectiveStats.Attack - stats.BaseStats.Attack,
		Defense:     stats.EffectiveStats.Defense - stats.BaseStats.Defense,
		Mobility:    stats.EffectiveStats.Mobility - stats.BaseStats.Mobility,
		Agility:     stats.EffectiveStats.Agility - stats.BaseStats.Agility,
		MaxHealth:   stats.EffectiveStats.MaxHealth - stats.BaseStats.MaxHealth,
		MaxEnergy:   stats.EffectiveStats.MaxEnergy - stats.BaseStats.MaxEnergy,
		SightRange:  stats.EffectiveStats.SightRange - stats.BaseStats.SightRange,
		AttackRange: stats.EffectiveStats.AttackRange - stats.BaseStats.AttackRange,
	}, nil
}

func loadActiveSetBonuses(setCounts map[int64]int, setCodeByID map[int64]string, setNameByID map[int64]string, legacyCounts map[int64]int) ([]ActiveEquipmentSetBonus, error) {
	if len(setCounts) == 0 {
		return []ActiveEquipmentSetBonus{}, nil
	}

	setIDs := make([]int64, 0, len(setCounts))
	for setID := range setCounts {
		setIDs = append(setIDs, setID)
	}
	sort.Slice(setIDs, func(i, j int) bool {
		return setIDs[i] < setIDs[j]
	})

	active := []ActiveEquipmentSetBonus{}
	for _, setID := range setIDs {
		rows, err := DB.Query(`
			SELECT
				pieces_required,
				description,
				attack_bonus,
				defense_bonus,
				mobility_bonus,
				agility_bonus,
				max_health_bonus,
				max_energy_bonus,
				sight_range_bonus,
				attack_range_bonus
			FROM item_set_bonuses
			WHERE set_id = $1
			  AND pieces_required <= $2
			ORDER BY pieces_required
		`, setID, setCounts[setID])
		if err != nil {
			return nil, fmt.Errorf("loadActiveSetBonuses query: %w", err)
		}
		for rows.Next() {
			bonus := ActiveEquipmentSetBonus{
				SetCode: setCodeByID[setID],
				SetName: setNameByID[setID],
				Pieces:  setCounts[setID],
			}
			if err := rows.Scan(
				&bonus.PiecesRequired,
				&bonus.Description,
				&bonus.Bonuses.Attack,
				&bonus.Bonuses.Defense,
				&bonus.Bonuses.Mobility,
				&bonus.Bonuses.Agility,
				&bonus.Bonuses.MaxHealth,
				&bonus.Bonuses.MaxEnergy,
				&bonus.Bonuses.SightRange,
				&bonus.Bonuses.AttackRange,
			); err != nil {
				rows.Close()
				return nil, fmt.Errorf("loadActiveSetBonuses scan: %w", err)
			}
			if mainEquipmentSet(bonus.SetCode) && legacyCounts[setID] < bonus.PiecesRequired {
				// New main items already carry HP/ATK/DEF/Reflex. Retain unrelated movement bonuses.
				bonus.Bonuses.Attack, bonus.Bonuses.Defense, bonus.Bonuses.MaxHealth, bonus.Bonuses.Agility = 0, 0, 0, 0
				if bonus.Bonuses.Mobility == 0 && bonus.Bonuses.SightRange == 0 && bonus.Bonuses.AttackRange == 0 {
					continue
				}
				bonus.Description = fmt.Sprintf("Mobility +%d; Sight +%d; Range +%d", bonus.Bonuses.Mobility, bonus.Bonuses.SightRange, bonus.Bonuses.AttackRange)
			}
			active = append(active, bonus)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("loadActiveSetBonuses rows: %w", err)
		}
		rows.Close()
	}
	return active, nil
}
