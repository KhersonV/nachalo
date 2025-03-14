//====================================
// gameservice/game/mapgen.go
//====================================

package game

import (
	"errors"
	"math/rand"
	"time"
)

// TileType определяет тип тайла на карте.
type TileType rune

const (
	Border    TileType = '1' // Граница карты (непроходимый)
	Walkable  TileType = '0' // Проходимый тайл
	Obstacle  TileType = ' ' // Непроходимый тайл (препятствие)
	Portal    TileType = 'p' // Портал (выход)
	StartTile TileType = 'P' // Стартовая позиция
)

// MapConfig задаёт параметры генерации карты.
type MapConfig struct {
	TotalPlayers int     // Общее количество игроков
	TeamsCount   int     // Количество команд
	WalkableProb float64 // Вероятность, что тайл будет проходимым
	ResourceProb float64 // Вероятность появления ресурса на проходимом тайле
	MonsterProb  float64 // Вероятность появления монстра на проходимом тайле
}

// ResourceData – структура для хранения данных ресурса.
type ResourceData struct {
	ID          int            `json:"id"`
	Type        string         `json:"type"`        // тип ресурса (например, "wood", "food")
	Description string         `json:"description"` // описание ресурса
	Effect      map[string]int `json:"effect"`      // эффекты или бонусы (например, {"energy": 10})
	Image       string         `json:"image"`       // путь к изображению ресурса
	// Дополнительные поля можно добавить
}

// MonsterData – структура для хранения данных монстра.
type MonsterData struct {
	ID              int    `json:"id"`
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
	// Дополнительные поля можно добавить
}

// FullCell – полная информация о клетке карты.
// Добавлены поля CellID (уникальный идентификатор) и IsPlayer (указывает, находится ли игрок в клетке).
type FullCell struct {
	CellID   int           `json:"cell_id"`            // Уникальный идентификатор клетки
	X        int           `json:"x"`                  // Координата X
	Y        int           `json:"y"`                  // Координата Y
	TileCode int           `json:"tileCode"`           // Код тайла (например, Walkable, Border, Portal, 'R', 'M', 'P')
	Resource *ResourceData `json:"resource,omitempty"` // Данные ресурса (если клетка содержит ресурс)
	Monster  *MonsterData  `json:"monster,omitempty"`  // Данные монстра (если клетка содержит монстра)
	IsPortal bool          `json:"isPortal,omitempty"` // Флаг, что клетка является порталом
	IsPlayer bool          `json:"isPlayer,omitempty"` // Флаг, что в клетке находится игрок
}

// GenerateFullMap генерирует полную карту, фиксируя размещение ресурсов и монстров.
// Функция возвращает срез FullCell, ширину, высоту, массив стартовых позиций и позицию портала.
func GenerateFullMap(cfg MapConfig, resources []ResourceData, monsters []MonsterData) ([]FullCell, int, int, [][2]int, [2]int, error) {
	// Проверка входных параметров.
	if cfg.TotalPlayers < 1 {
		return nil, 0, 0, nil, [2]int{}, errors.New("общее количество игроков должно быть >= 1")
	}
	if cfg.TeamsCount < 1 {
		return nil, 0, 0, nil, [2]int{}, errors.New("количество команд должно быть >= 1")
	}

	// Определяем размеры карты: ширина и высота равны 15 * TotalPlayers.
	width := 15 * cfg.TotalPlayers
	height := 15 * cfg.TotalPlayers

	// Инициализация генератора случайных чисел.
	rand.Seed(time.Now().UnixNano())

	// Генерация базовой карты в виде двумерного среза tileCode.
	grid := make([][]int, height)
	for y := 0; y < height; y++ {
		grid[y] = make([]int, width)
		for x := 0; x < width; x++ {
			if x == 0 || y == 0 || x == width-1 || y == height-1 {
				grid[y][x] = int(Border)
			} else {
				if rand.Float64() < cfg.WalkableProb {
					grid[y][x] = int(Walkable)
				} else {
					grid[y][x] = int(Obstacle)
				}
			}
		}
	}

	// Функция для размещения специальных тайлов: стартовых позиций и портала.
	placeSpecialTiles := func(grid [][]int, teamsCount int) ([][2]int, [2]int, error) {
		var candidates []struct{ x, y int }
		// Собираем все проходимые тайлы (Walkable).
		for y := 1; y < height-1; y++ {
			for x := 1; x < width-1; x++ {
				if grid[y][x] == int(Walkable) {
					candidates = append(candidates, struct{ x, y int }{x, y})
				}
			}
		}
		// Для размещения необходимо как минимум teamsCount+1 проходимых тайлов.
		if len(candidates) < teamsCount+1 {
			return nil, [2]int{}, errors.New("недостаточно проходимых тайлов для размещения специальных точек")
		}
		// Перемешиваем кандидатов.
		rand.Shuffle(len(candidates), func(i, j int) {
			candidates[i], candidates[j] = candidates[j], candidates[i]
		})
		var starts [][2]int
		// Первые teamsCount кандидатов назначаются стартовыми позициями.
		for i := 0; i < teamsCount; i++ {
			pt := candidates[i]
			grid[pt.y][pt.x] = int(StartTile)
			starts = append(starts, [2]int{pt.x, pt.y})
		}
		// Следующий кандидат назначается порталом.
		portalCandidate := candidates[teamsCount]
		grid[portalCandidate.y][portalCandidate.x] = int(Portal)
		portalPos := [2]int{portalCandidate.x, portalCandidate.y}
		return starts, portalPos, nil
	}

	startPositions, portalPos, err := placeSpecialTiles(grid, cfg.TeamsCount)
	if err != nil {
		return nil, 0, 0, nil, [2]int{}, err
	}

	// Формируем полную карту.
	var fullCells []FullCell
	cellIDCounter := 1
	// Счётчики для уникальных ID копий ресурсов и монстров.
	resourceCounter := 1
	monsterCounter := 1

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			tileCode := grid[y][x]
			var res *ResourceData = nil
			var mon *MonsterData = nil
			isPortal := false

			// Если клетка исходно проходимая (Walkable), случайно решаем, появится ли в ней ресурс или монстр.
			if tileCode == int(Walkable) {
				r := rand.Float64()
				if r < cfg.MonsterProb && len(monsters) > 0 {
					chosen := monsters[rand.Intn(len(monsters))]
					// Генерируем уникальный ID для этой копии.
					newID := chosen.ID*1000000 + monsterCounter
					monsterCounter++
					mon = &MonsterData{
						ID:              newID,
						Name:            chosen.Name,
						Type:            chosen.Type,
						Health:          chosen.Health,
						MaxHealth:       chosen.MaxHealth,
						Attack:          chosen.Attack,
						Defense:         chosen.Defense,
						Speed:           chosen.Speed,
						Maneuverability: chosen.Maneuverability,
						Vision:          chosen.Vision,
						Image:           chosen.Image,
					}
					// Обновляем tileCode, чтобы отметить наличие монстра.
					tileCode = int('M')
				} else if r < cfg.MonsterProb+cfg.ResourceProb && len(resources) > 0 {
					chosen := resources[rand.Intn(len(resources))]
					newID := chosen.ID*1000000 + resourceCounter
					resourceCounter++
					res = &ResourceData{
						ID:          newID,
						Type:        chosen.Type,
						Description: chosen.Description,
						Effect:      chosen.Effect,
						Image:       chosen.Image,
					}
					tileCode = int('R')
				}
			}

			// Если клетка имеет код портала, устанавливаем флаг.
			if tileCode == int(Portal) || tileCode == 112 {
				isPortal = true
			}

			fullCells = append(fullCells, FullCell{
				CellID:   cellIDCounter,
				X:        x,
				Y:        y,
				TileCode: tileCode,
				Resource: res,
				Monster:  mon,
				IsPortal: isPortal,
				IsPlayer: false,
			})
			cellIDCounter++
		}
	}

	return fullCells, width, height, startPositions, portalPos, nil
}
