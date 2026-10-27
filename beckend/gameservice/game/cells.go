// ====================================
// gameservice/game/cells.go
// ====================================

package game

import "math/rand"

// TileType определяет тип тайла на карте.
type TileType rune

const (
	Border    TileType = '1' // Граница карты (непроходимый)
	Walkable  TileType = '0' // Проходимый тайл
	Obstacle  TileType = ' ' // Непроходимый тайл (препятствие)
	Portal    TileType = 'p' // Портал (выход)
	StartTile TileType = 'P' // Стартовая позиция
)

const (
	BarbelTile TileType = 'B' // Символ для бочки
)

// ResourceData – структура для хранения данных ресурса.
type ProfessionNode struct {
	ProfessionName string `json:"professionName"`
	Category       string `json:"category"`
	Class          string `json:"class"`
	Profession     string `json:"profession"`
}
type ResourceData struct {
	ProfessionNode *ProfessionNode `json:"professionNode,omitempty"`
	ID             int             `json:"id"`
	Type           string          `json:"type"`                // тип ресурса (например, "wood", "food")
	Description    string          `json:"description"`         // описание ресурса
	Effect         map[string]int  `json:"effect"`              // эффекты или бонусы (например, {"energy": 10})
	Image          string          `json:"image"`               // путь к изображению ресурса
	ItemType       string          `json:"item_type,omitempty"` // "resource" или "artifact" (для дропа на клетку)
}

// MonsterData – структура для хранения данных монстра.
type MonsterData struct {
	ID              int    `json:"id"`              // виртуальный id (оставь для совместимости)
	DBInstanceID    int    `json:"db_instance_id"`  // реальный id из match_monsters!
	Name            string `json:"name"`            // имя монстра (например, "Goblin")
	Type            string `json:"type"`            // тип монстра (например, "aggressive")
	Health          int    `json:"health"`          // текущее здоровье
	MaxHealth       int    `json:"max_health"`      // максимальное здоровье
	Attack          int    `json:"attack"`          // показатель атаки
	Defense         int    `json:"defense"`         // показатель защиты
	Speed           int    `json:"speed"`           // скорость
	Maneuverability int    `json:"maneuverability"` // маневренность
	Vision          int    `json:"vision"`          // радиус видения
	Image           string `json:"image"`           // путь к изображению монстра
}

// FullCell – полная информация о клетке карты.
type FullCell struct {
	CellID                int           `json:"cell_id"`  // Уникальный идентификатор клетки
	X                     int           `json:"x"`        // Координата X
	Y                     int           `json:"y"`        // Координата Y
	TileCode              int           `json:"tileCode"` // Код тайла
	Resource              *ResourceData `json:"resource"` // Ресурс (если присутствует и НЕ является бочкой)
	Barbel                *ResourceData `json:"barbel"`   // Бочка (если присутствует)
	Monster               *MonsterData  `json:"monster"`  // Монстр (если присутствует)
	IsPortal              bool          `json:"isPortal"` // Портал
	IsPlayer              bool          `json:"isPlayer"` // Флаг наличия игрока
	StructureType         string        `json:"structure_type,omitempty"`
	StructureOwnerUserID  int           `json:"structure_owner_user_id,omitempty"`
	StructureHealth       int           `json:"structure_health,omitempty"`
	StructureDefense      int           `json:"structure_defense,omitempty"`
	StructureAttack       int           `json:"structure_attack,omitempty"`
	StructureEnergy       int           `json:"structure_energy,omitempty"`
	IsUnderConstruction   bool          `json:"is_under_construction"`
	ConstructionTurnsLeft int           `json:"construction_turns_left,omitempty"`
}

var rnd = rand.Float64

// SetRnd позволяет установить свою функцию генерации [0,1)
func SetRnd(f func() float64) {
	if f == nil {
		rnd = rand.Float64
	} else {
		rnd = f
	}
}

// OpenBarbel реализует логику открытия бочки.
func OpenBarbel(
	cell FullCell,
	resources []ResourceData,
	artifacts []ResourceData,

) (interface{}, error) {
	if cell.Barbel == nil {
		return nil, nil
	}

	r := rnd()
	// Параметры урона (можно вынести в константы или Config)
	const minDamage = 3
	const maxDamage = 5

	// One category roll per barrel; no per-item or fallback artifact rolls.
	if r < float64(BarrelDamageChancePercent)/100 {
		// Рассчитываем случайный урон в диапазоне [minDamage, maxDamage]
		dmg := minDamage + rand.Intn(maxDamage-minDamage+1)
		return DamageEvent{SourceType: ResultSourceEnvironment, Amount: dmg}, nil
	}
	// Preserve the ordinary resource category.
	if r < float64(BarrelDamageChancePercent+BarrelResourceChancePercent)/100 {
		// Собираем всех ресурсов, кроме того, что был в бочке
		var candidates []ResourceData
		for _, res := range resources {
			if cell.Barbel != nil && res.ID == cell.Barbel.ID && res.Type == cell.Barbel.Type {
				// пропускаем — это сама бочка
				continue
			}
			candidates = append(candidates, res)
		}
		if len(candidates) == 0 {
			return EmptyBarrelEvent{}, nil
		}
		chosen := candidates[rand.Intn(len(candidates))]
		return chosen, nil
	}

	// Only this interval yields an ordinary artifact. Missing pools never
	// increase another category's probability.
	if r < float64(BarrelDamageChancePercent+BarrelResourceChancePercent+BarrelArtifactChancePercent())/100 && len(artifacts) > 0 {
		chosen := artifacts[rand.Intn(len(artifacts))]
		return chosen, nil
	}

	return EmptyBarrelEvent{}, nil
}
