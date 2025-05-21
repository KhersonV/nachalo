// ====================================
// gameservice/game/mapgen.go
// ====================================

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

const (
	BarbelTile TileType = 'B' // Символ для бочки
)

// MapConfig задаёт параметры генерации карты.
type MapConfig struct {
	TotalPlayers int     // Общее количество игроков
	TeamsCount   int     // Количество команд
	WalkableProb float64 // Вероятность, что тайл будет проходимым
	// Вероятности указываются как доли (например, 0.05 = 5%)
	MonsterProb  float64 // Вероятность появления монстра
	BarbelProb   float64 // Вероятность появления бочки
	ResourceProb float64 // Вероятность появления обычного ресурса
}

// ResourceData – структура для хранения данных ресурса.
type ResourceData struct {
	ID          int            `json:"id"`
	Type        string         `json:"type"`        // тип ресурса (например, "wood", "food")
	Description string         `json:"description"` // описание ресурса
	Effect      map[string]int `json:"effect"`      // эффекты или бонусы (например, {"energy": 10})
	Image       string         `json:"image"`       // путь к изображению ресурса
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
}

// FullCell – полная информация о клетке карты.
type FullCell struct {
	CellID   int           `json:"cell_id"`  // Уникальный идентификатор клетки
	X        int           `json:"x"`        // Координата X
	Y        int           `json:"y"`        // Координата Y
	TileCode int           `json:"tileCode"` // Код тайла
	Resource *ResourceData `json:"resource"` // Ресурс (если присутствует и НЕ является бочкой)
	Barbel   *ResourceData `json:"barbel"`   // Бочка (если присутствует)
	Monster  *MonsterData  `json:"monster"`  // Монстр (если присутствует)
	IsPortal bool          `json:"isPortal"` // Портал
	IsPlayer bool          `json:"isPlayer"` // Флаг наличия игрока
}







// проверяет, достижима ли клетка (tx,ty) из (sx,sy)
func isReachable(grid [][]int, sx, sy, tx, ty int) bool {
    h, w := len(grid), len(grid[0])
    visited := make([][]bool, h)
    for i := range visited {
        visited[i] = make([]bool, w)
    }
    type pt struct{ x, y int }
    q := []pt{{sx, sy}}
    visited[sy][sx] = true
    dirs := []pt{{1,0},{-1,0},{0,1},{0,-1}}
    for len(q) > 0 {
        p := q[0]; q = q[1:]
        if p.x == tx && p.y == ty {
            return true
        }
        for _, d := range dirs {
            nx, ny := p.x + d.x, p.y + d.y
            if nx > 0 && ny > 0 && nx < w-1 && ny < h-1 &&
               !visited[ny][nx] && grid[ny][nx] == int(Walkable) {
                visited[ny][nx] = true
                q = append(q, pt{nx, ny})
            }
        }
    }
    return false
}

func placeSpecialTiles(grid [][]int, teamsCount int) ([][2]int, [2]int, error) {
    var candidates []struct{ x, y int }
    h, w := len(grid), len(grid[0])
    for y := 1; y < h-1; y++ {
        for x := 1; x < w-1; x++ {
            if grid[y][x] == int(Walkable) {
                candidates = append(candidates, struct{ x, y int }{x, y})
            }
        }
    }
    if len(candidates) < teamsCount+1 {
        return nil, [2]int{}, errors.New("недостаточно проходимых тайлов для специальных точек")
    }

    // Попытки подобрать связный набор
    rand.Shuffle(len(candidates), func(i, j int) {
        candidates[i], candidates[j] = candidates[j], candidates[i]
    })
    for startIdx := 0; startIdx <= len(candidates)-teamsCount-1; startIdx++ {
        // берём блок подряд: [startIdx..startIdx+teamsCount-1] – старты, [startIdx+teamsCount] – портал
        var starts [][2]int
        for i := 0; i < teamsCount; i++ {
            pt := candidates[startIdx+i]
            starts = append(starts, [2]int{pt.x, pt.y})
        }
        portal := candidates[startIdx+teamsCount]
        // проверяем, что из любого старта до портала есть путь
        ok := false
        for _, st := range starts {
            if isReachable(grid, st[0], st[1], portal.x, portal.y) {
                ok = true
                break
            }
        }
        if !ok {
            continue // пробуем следующий кусок
        }
        // разметить их на grid
        for _, st := range starts {
            grid[st[1]][st[0]] = int(StartTile)
        }
        grid[portal.y][portal.x] = int(Portal)
        return starts, [2]int{portal.x, portal.y}, nil
    }

    return nil, [2]int{}, errors.New("не удалось найти связные старт+портал точки")
}





// GenerateFullMap генерирует полную карту, фиксируя размещение ресурсов и монстров.
// Возвращает срез FullCell, ширину, высоту, массив стартовых позиций и позицию портала.
func GenerateFullMap(cfg MapConfig, resources []ResourceData, monsters []MonsterData) ([]FullCell, int, int, [][2]int, [2]int, error) {
	if cfg.TotalPlayers < 1 {
		return nil, 0, 0, nil, [2]int{}, errors.New("общее количество игроков должно быть >= 1")
	}
	if cfg.TeamsCount < 1 {
		return nil, 0, 0, nil, [2]int{}, errors.New("количество команд должно быть >= 1")
	}

	width := 15 * cfg.TotalPlayers
	height := 15 * cfg.TotalPlayers

	rand.Seed(time.Now().UnixNano())

	// Генерируем базовую сетку: границы – непроходимые, внутри случайно Walkable или Obstacle.
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

	startPositions, portalPos, err := placeSpecialTiles(grid, cfg.TeamsCount)
	if err != nil {
		return nil, 0, 0, nil, [2]int{}, err
	}

	monsterCounter := 1
	var fullCells []FullCell
	cellIDCounter := 1

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			tileCode := grid[y][x]
			var res *ResourceData = nil
			var barbel *ResourceData = nil
			var mon *MonsterData = nil
			isPortal := false

			// Если клетка исходно проходимая, решаем, появляется ли что-либо на ней.
			if tileCode == int(Walkable) {
				// Сначала проверяем шанс появления монстра
				r := rand.Float64()
				if r < cfg.MonsterProb && len(monsters) > 0 {
					chosen := monsters[rand.Intn(len(monsters))]
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
					tileCode = int('M')
				} else {
					// Если монстр не появился, генерируем новое случайное число для решения между бочкой и ресурсом.
					r2 := rand.Float64()
					// Если r2 меньше BarbelProb, то обязательно появляется бочка.
					if r2 < cfg.BarbelProb && len(resources) > 0 {
						// Находим ресурс с ID == 6 (предполагается, что он отвечает за бочку)
						var barrel *ResourceData
						for _, r := range resources {
							if r.ID == 6 {
								barrel = &ResourceData{
									ID:          r.ID,
									Type:        r.Type,
									Description: r.Description,
									Effect:      r.Effect,
									Image:       r.Image,
								}
								break
							}
						}
						// Если ресурс для бочки найден, устанавливаем его и меняем tileCode
						if barrel != nil {
							barbel = barrel
							tileCode = int(BarbelTile)
						}
					} else if r2 < cfg.BarbelProb+cfg.ResourceProb && len(resources) > 0 {
						// Иначе, если r2 попадает в диапазон для обычного ресурса, выбираем ресурс, у которого ID != 6.
						var normalResource *ResourceData
						// Попытаемся выбрать случайный ресурс, пока не найдём тот, у которого ID != 6.
						for i := 0; i < 10; i++ {
							candidate := resources[rand.Intn(len(resources))]
							if candidate.ID != 6 {
								normalResource = &ResourceData{
									ID:          candidate.ID,
									Type:        candidate.Type,
									Description: candidate.Description,
									Effect:      candidate.Effect,
									Image:       candidate.Image,
								}
								break
							}
						}
						if normalResource != nil {
							res = normalResource
							tileCode = int('R')
						}
					}
					// Если ни одно из условий не выполнено, клетка остаётся просто Walkable (tileCode = '0')
				}
			}

			if tileCode == int(Portal) || tileCode == 112 {
				isPortal = true
			}

			// При генерации карты все поля явно устанавливаются:
			fullCells = append(fullCells, FullCell{
				CellID:   cellIDCounter,
				X:        x,
				Y:        y,
				TileCode: tileCode,
				Resource: res,
				Barbel:   barbel,
				Monster:  mon,
				IsPortal: isPortal,
				IsPlayer: false,
			})
			cellIDCounter++
		}
	}

	return fullCells, width, height, startPositions, portalPos, nil
}



// OpenBarbel реализует логику открытия бочки.
// В зависимости от случайного числа, из бочки может выпасть,
// обычный ресурс или артефакт. а может и урон нанести персонажу.
// types.go (или в том же файле до OpenBarbel)

type DamageEvent struct {
    Amount int
}

// OpenBarbel реализует логику открытия бочки.
// Раньше выпадал монстр, теперь вместо этого наносится урон персонажу.
func OpenBarbel(
    cell FullCell,
    resources []ResourceData,
    artifacts []ResourceData,

) (interface{}, error) {
    r := rand.Float64()
    // Параметры урона (можно вынести в константы или Config)
    const minDamage = 5
    const maxDamage = 80

    // 30% шанс получить урон вместо монстра
    if r < 0.3 {
        // Рассчитываем случайный урон в диапазоне [minDamage, maxDamage]
        dmg := minDamage + rand.Intn(maxDamage-minDamage+1)
        return DamageEvent{Amount: dmg}, nil
    }
    // 40% шанс выпадения ресурса
    if r < 0.3+0.4 && len(resources) > 0 {
        chosen := resources[rand.Intn(len(resources))]
        return chosen, nil
    }
    // Иначе артефакт
    if len(artifacts) > 0 {
        chosen := artifacts[rand.Intn(len(artifacts))]
        return chosen, nil
    }

    return nil, errors.New("не удалось определить результат открытия бочки")
}
