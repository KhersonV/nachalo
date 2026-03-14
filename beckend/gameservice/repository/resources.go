
// ======================================
// /gameservice/repository/resources.go
// ======================================

package repository

import (
	"encoding/json"
	"fmt"
	"strings"

	"gameservice/game"
)

func effectToBytes(value any) []byte {
	switch v := value.(type) {
	case []byte:
		return v
	case string:
		return []byte(v)
	case nil:
		return []byte("{}")
	default:
		b, _ := json.Marshal(v)
		return b
	}
}

// GetResourceByType возвращает ресурс по его типу (например food/water).
func GetResourceByType(resourceType string) (*game.ResourceData, error) {
	var rd game.ResourceData
	var effectValue any
	err := DB.QueryRow(
		`SELECT id, type, description, effect, image FROM resources WHERE type = $1 LIMIT 1`,
		resourceType,
	).Scan(&rd.ID, &rd.Type, &rd.Description, &effectValue, &rd.Image)
	if err != nil {
		return nil, fmt.Errorf("GetResourceByType: %w", err)
	}

	effectBytes := effectToBytes(effectValue)
	var effect map[string]int
	if err := json.Unmarshal(effectBytes, &effect); err != nil {
		effect = make(map[string]int)
	}
	rd.Effect = effect
	rd.Image = strings.Trim(rd.Image, "\"")
	return &rd, nil
}

// GetResourcesData извлекает данные ресурсов из таблицы resources
// и возвращает срез game.ResourceData.
func GetResourcesData() ([]game.ResourceData, error) {
	var resources []game.ResourceData
	rows, err := DB.Query(`SELECT id, type, description, effect, image FROM resources`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var rd game.ResourceData
		var effectValue any
		if err := rows.Scan(&rd.ID, &rd.Type, &rd.Description, &effectValue, &rd.Image); err != nil {
			return nil, err
		}
		effectBytes := effectToBytes(effectValue)
		var effect map[string]int
		if err := json.Unmarshal(effectBytes, &effect); err != nil {
			effect = make(map[string]int)
		}
		rd.Effect = effect
		rd.Image = strings.Trim(rd.Image, "\"")
		resources = append(resources, rd)
	}
	return resources, nil
}

// GetMonstersData извлекает данные монстров из таблицы monsters
// и возвращает срез game.MonsterData.
func GetMonstersData() ([]game.MonsterData, error) {
	var monsters []game.MonsterData
	rows, err := DB.Query(`SELECT id, name, type, health, max_health, attack, defense, speed, maneuverability, vision, image FROM monsters`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var md game.MonsterData
		if err := rows.Scan(&md.ID, &md.Name, &md.Type, &md.Health, &md.MaxHealth, &md.Attack, &md.Defense, &md.Speed, &md.Maneuverability, &md.Vision, &md.Image); err != nil {
			return nil, err
		}
		md.Image = strings.Trim(md.Image, "\"")
		monsters = append(monsters, md)
	}
	return monsters, nil
}


func GetArtifactsData() ([]game.ResourceData, error) {
    rows, err := DB.Query(`SELECT id, name AS type, description, bonus AS effect, image FROM artifacts`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var arts []game.ResourceData
    for rows.Next() {
        var rd game.ResourceData
        var effectBytes []byte
        if err := rows.Scan(&rd.ID, &rd.Type, &rd.Description, &effectBytes, &rd.Image); err != nil {
            return nil, err
        }
        // bonus в БД — это JSON-объект вида {"attack":5,...}
        var effect map[string]int
        if err := json.Unmarshal(effectBytes, &effect); err != nil {
            effect = make(map[string]int)
        }
        rd.Effect = effect
        rd.Image = strings.Trim(rd.Image, `"`)
        arts = append(arts, rd)
    }
    return arts, nil
}

