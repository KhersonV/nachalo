
// ===================================
// /gameservice/repository/monsters.go
// ===================================


package repository

import (
	"log"

	"gameservice/models"
)

// GetMonsterByID возвращает монстра по его ID
func GetMonsterByID(id int) (*models.Monster, error) {
	const q = `
		SELECT id, name, type, health, max_health, attack, defense,
		       speed, maneuverability, vision, image, created_at
		FROM monsters
		WHERE id = $1
	`
	mon := &models.Monster{}
	err := DB.QueryRow(q, id).Scan(
		&mon.ID,
		&mon.Name,
		&mon.Type,
		&mon.Health,
		&mon.MaxHealth,
		&mon.Attack,
		&mon.Defense,
		&mon.Speed,
		&mon.Maneuverability,
		&mon.Vision,
		&mon.Image,
		&mon.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return mon, nil
}

// UpdateMonster сохраняет изменения монстра (например, понижает HP)
func UpdateMonster(mon *models.Monster) error {
	const q = `
		UPDATE monsters
		SET health = $1
		WHERE id = $2
	`
	if _, err := DB.Exec(q, mon.Health, mon.ID); err != nil {
		log.Printf("UpdateMonster error: %v", err)
		return err
	}
	return nil
}
