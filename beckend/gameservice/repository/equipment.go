package repository

import (
	"database/sql"
	"errors"
	"fmt"
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
	Attack      int `json:"attack"`
	Defense     int `json:"defense"`
	Mobility    int `json:"mobility"`
	Agility     int `json:"agility"`
	MaxHealth   int `json:"maxHealth"`
	MaxEnergy   int `json:"maxEnergy"`
	SightRange  int `json:"sightRange"`
	AttackRange int `json:"attackRange"`
}

type EquipmentStats struct {
	MaxHealth   int  `json:"maxHealth"`
	MaxEnergy   int  `json:"maxEnergy"`
	Attack      int  `json:"attack"`
	Defense     int  `json:"defense"`
	Mobility    int  `json:"mobility"`
	Agility     int  `json:"agility"`
	SightRange  int  `json:"sightRange"`
	IsRanged    bool `json:"isRanged"`
	AttackRange int  `json:"attackRange"`
}

type EquipmentItem struct {
	InstanceID       string           `json:"instanceId"`
	TemplateID       int64            `json:"templateId"`
	Code             string           `json:"code"`
	Name             string           `json:"name"`
	SetID            *int64           `json:"-"`
	SetCode          string           `json:"setCode,omitempty"`
	SetName          string           `json:"setName,omitempty"`
	Slot             string           `json:"slot"`
	ItemType         string           `json:"itemType"`
	Handedness       string           `json:"handedness"`
	Rarity           string           `json:"rarity"`
	ClassRestriction string           `json:"classRestriction,omitempty"`
	LevelRequirement int              `json:"levelRequirement"`
	ImageURL         string           `json:"imageUrl"`
	Bonuses          EquipmentBonuses `json:"bonuses"`
	Status           string           `json:"status"`
	Version          int              `json:"version"`
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
	CharacterID      int                       `json:"characterId"`
	HeroClassID      string                    `json:"heroClassId"`
	BaseStats        EquipmentStats            `json:"baseStats"`
	EffectiveStats   EquipmentStats            `json:"effectiveStats"`
	Equipped         map[string]*EquipmentItem `json:"equipped"`
	ActiveSetBonuses []ActiveEquipmentSetBonus `json:"activeSetBonuses"`
}

type equipmentItemLock struct {
	EquipmentItem
	OwnerUserID        int
	CurrentCharacterID sql.NullInt64
	IsLocked           bool
}

type equippedSlotLock struct {
	Slot           string
	ItemInstanceID string
	OwnerUserID    int
	Handedness     string
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

func equipmentStatsFromCharacter(character *PlayerCharacter) EquipmentStats {
	return EquipmentStats{
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
	stats.Attack += bonuses.Attack
	stats.Defense += bonuses.Defense
	stats.Mobility += bonuses.Mobility
	stats.Agility += bonuses.Agility
	stats.MaxHealth += bonuses.MaxHealth
	stats.MaxEnergy += bonuses.MaxEnergy
	stats.SightRange += bonuses.SightRange
	stats.AttackRange += bonuses.AttackRange
}

func scanEquipmentItem(scan func(dest ...interface{}) error) (*EquipmentItem, error) {
	var item EquipmentItem
	var setID sql.NullInt64
	var setCode sql.NullString
	var setName sql.NullString
	var classRestriction sql.NullString
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
		&item.Status,
		&item.Version,
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
	return &item, nil
}

func equipmentItemSelectClause() string {
	return `
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
		ii.status,
		ii.version
	`
}

func GetEquipmentInventoryForUser(userID int) ([]EquipmentItem, error) {
	rows, err := DB.Query(`
		SELECT `+equipmentItemSelectClause()+`
		FROM item_instances ii
		JOIN item_templates it ON it.id = ii.template_id
		LEFT JOIN item_sets s ON s.id = it.set_id
		WHERE ii.owner_user_id = $1
		  AND ii.status = 'inventory'
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

func GetCharacterEquipment(characterID int) (map[string]*EquipmentItem, error) {
	rows, err := DB.Query(`
		SELECT `+equipmentItemSelectClause()+`
		FROM character_equipment ce
		JOIN item_instances ii ON ii.id = ce.item_instance_id
		JOIN item_templates it ON it.id = ii.template_id
		LEFT JOIN item_sets s ON s.id = it.set_id
		WHERE ce.character_id = $1
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
		allDest := append(dest, &row.OwnerUserID, &row.CurrentCharacterID, &row.IsLocked)
		return tx.QueryRow(`
			SELECT `+equipmentItemSelectClause()+`,
				ii.owner_user_id,
				ii.current_character_id,
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

func GrantItemInstanceToUser(userID int, templateCode string, source string) (*EquipmentItem, error) {
	templateCode = strings.TrimSpace(templateCode)
	if templateCode == "" {
		return nil, ErrEquipmentInvalidTemplate
	}
	source = normalizeEquipmentSource(source)

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
		WHERE code = $3
		RETURNING id::text
	`, userID, source, templateCode).Scan(&itemInstanceID); err != nil {
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
	setCodeByID := map[int64]string{}
	setNameByID := map[int64]string{}
	for _, item := range equipped {
		addEquipmentBonuses(&effectiveStats, item.Bonuses)
		if item.SetID != nil {
			setID := *item.SetID
			setCounts[setID]++
			setCodeByID[setID] = item.SetCode
			setNameByID[setID] = item.SetName
		}
	}

	activeSetBonuses, err := loadActiveSetBonuses(setCounts, setCodeByID, setNameByID)
	if err != nil {
		return nil, err
	}
	for _, bonus := range activeSetBonuses {
		addEquipmentBonuses(&effectiveStats, bonus.Bonuses)
	}

	return &CharacterEffectiveStats{
		CharacterID:      character.ID,
		HeroClassID:      character.HeroClassID,
		BaseStats:        baseStats,
		EffectiveStats:   effectiveStats,
		Equipped:         equipped,
		ActiveSetBonuses: activeSetBonuses,
	}, nil
}

func loadActiveSetBonuses(setCounts map[int64]int, setCodeByID map[int64]string, setNameByID map[int64]string) ([]ActiveEquipmentSetBonus, error) {
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
