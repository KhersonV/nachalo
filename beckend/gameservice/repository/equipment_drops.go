package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
)

type DroppedEquipment struct {
	InstanceID   string
	OwnerUserID  int
	TemplateID   int64
	TemplateCode string
	Name         string
	Rarity       string
	ImageURL     string
	Slot         string
	ItemType     string
	ClassID      string
	SetCode      string
	SetName      string
}

var pickRandomEquipmentDropIndex = func(count int) int {
	if count <= 0 {
		return -1
	}
	return rand.Intn(count)
}

type equipmentDropTemplateCandidate struct {
	TemplateID   int64
	TemplateCode string
	Name         string
	Rarity       string
	ImageURL     string
	Slot         string
	ItemType     string
	ClassID      string
	SetCode      string
	SetName      string
}

// GrantRandomEquipmentDrop creates one unique dropped item from the global
// green starter-set pool. Duplicates are allowed: each successful monster drop
// creates a fresh item_instances UUID even if the user already owns that
// template.
//
// TODO(equipment-drops): move this catalog choice to monster-specific drop
// tables keyed by monster_ref_id/type/rarity when more equipment exists.
func GrantRandomEquipmentDrop(userID int) (*DroppedEquipment, error) {
	return grantRandomEquipmentDrop(userID, "")
}

// GrantRandomEquipmentDropForClass is kept for focused dev/test tooling. Monster
// drops should use GrantRandomEquipmentDrop so the killer class never limits the
// drop pool.
func GrantRandomEquipmentDropForClass(userID int, classID string) (*DroppedEquipment, error) {
	classID = NormalizeHeroClassID(classID)
	if !IsKnownHeroClassID(classID) {
		return nil, ErrEquipmentClassRestricted
	}
	return grantRandomEquipmentDrop(userID, classID)
}

func grantRandomEquipmentDrop(userID int, classID string) (*DroppedEquipment, error) {
	tx, err := DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("GrantRandomEquipmentDrop begin tx: %w", err)
	}
	defer tx.Rollback()

	var lockedUserID int
	if err := tx.QueryRow(`
		SELECT user_id
		FROM player_profiles
		WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(&lockedUserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPlayerNotFound
		}
		return nil, fmt.Errorf("GrantRandomEquipmentDrop lock profile: %w", err)
	}

	rows, err := tx.Query(`
		SELECT
			it.id,
			it.code,
			it.name,
			it.rarity,
			it.image_url,
			it.slot,
			it.item_type,
			COALESCE(it.class_restriction, ''),
			COALESCE(s.code, ''),
			COALESCE(s.name, '')
		FROM item_templates it
		JOIN item_sets s ON s.id = it.set_id
		WHERE it.rarity = 'green'
		  AND s.rarity = 'green'
		  AND s.code IN ('aegiswarden_set', 'bloodroot_set', 'greenwisp_set', 'sagecloth_set')
		  AND it.class_restriction = s.class_restriction
		  AND ($1 = '' OR it.class_restriction = $1)
		ORDER BY s.class_restriction, it.slot, it.name
	`, classID)
	if err != nil {
		return nil, fmt.Errorf("GrantRandomEquipmentDrop select templates: %w", err)
	}
	defer rows.Close()

	candidates := make([]equipmentDropTemplateCandidate, 0, 8)
	for rows.Next() {
		var candidate equipmentDropTemplateCandidate
		if err := rows.Scan(
			&candidate.TemplateID,
			&candidate.TemplateCode,
			&candidate.Name,
			&candidate.Rarity,
			&candidate.ImageURL,
			&candidate.Slot,
			&candidate.ItemType,
			&candidate.ClassID,
			&candidate.SetCode,
			&candidate.SetName,
		); err != nil {
			return nil, fmt.Errorf("GrantRandomEquipmentDrop scan template: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GrantRandomEquipmentDrop template rows: %w", err)
	}
	rows.Close()

	if len(candidates) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("GrantRandomEquipmentDrop empty commit: %w", err)
		}
		return nil, nil
	}

	index := pickRandomEquipmentDropIndex(len(candidates))
	if index < 0 || index >= len(candidates) {
		return nil, fmt.Errorf("GrantRandomEquipmentDrop pick template: index %d out of %d", index, len(candidates))
	}
	selected := candidates[index]

	var itemInstanceID string
	if err := tx.QueryRow(`
		INSERT INTO item_instances (template_id, owner_user_id, source)
		VALUES ($1, $2, 'drop')
		RETURNING id::text
	`, selected.TemplateID, userID).Scan(&itemInstanceID); err != nil {
		return nil, fmt.Errorf("GrantRandomEquipmentDrop insert item: %w", err)
	}

	toUserID := userID
	if err := insertItemInstanceEventTx(tx, itemInstanceID, "created", nil, &toUserID, nil, nil, ""); err != nil {
		return nil, fmt.Errorf("GrantRandomEquipmentDrop insert created event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("GrantRandomEquipmentDrop commit: %w", err)
	}

	return &DroppedEquipment{
		InstanceID:   itemInstanceID,
		OwnerUserID:  userID,
		TemplateID:   selected.TemplateID,
		TemplateCode: selected.TemplateCode,
		Name:         selected.Name,
		Rarity:       selected.Rarity,
		ImageURL:     selected.ImageURL,
		Slot:         selected.Slot,
		ItemType:     selected.ItemType,
		ClassID:      selected.ClassID,
		SetCode:      selected.SetCode,
		SetName:      selected.SetName,
	}, nil
}
