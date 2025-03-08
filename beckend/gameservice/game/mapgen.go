

package game

import (
	"errors"
	"math/rand"
	"time"
)

// TileType определяет тип тайла на карте.
type TileType rune

const (
	Border    TileType = '1' // Граница карты (непроходимый тайл)
	Walkable  TileType = '0' // Проходимый тайл, на котором могут появляться объекты
	Obstacle  TileType = ' ' // Непроходимый тайл (препятствие)
	Portal    TileType = 'p' // Портал для завершения инстанса (ровно один)
	StartTile TileType = 'P' // Стартовая позиция для команды
	// Дополнительные символы можно добавить по необходимости.
)

// MapGrid – карта в виде двумерного среза тайлов.
type MapGrid [][]TileType

// MapConfig задаёт параметры генерации карты.
type MapConfig struct {
	TotalPlayers  int     // Общее количество игроков в матче (например, 10 для 5х5).
	TeamsCount    int     // Количество команд (например, 2).
	WalkableProb  float64 // Вероятность, что внутренний тайл будет проходимым ("0").
	ResourceProb  float64 // Вероятность появления ресурса на проходимом тайле.
	MonsterProb   float64 // Вероятность появления монстра на проходимом тайле.
	// Размер карты будет вычисляться как (15 * TotalPlayers) по ширине и высоте.
}

// GenerateMap генерирует карту согласно конфигурации.
// Размер карты вычисляется как (15 * TotalPlayers) x (15 * TotalPlayers).
// Возвращает карту, список стартовых позиций для команд и позицию портала.
func GenerateMap(cfg MapConfig) (MapGrid, [][2]int, [2]int, error) {
	if cfg.TotalPlayers < 1 {
		return nil, nil, [2]int{}, errors.New("общее количество игроков должно быть >= 1")
	}
	if cfg.TeamsCount < 1 {
		return nil, nil, [2]int{}, errors.New("количество команд должно быть >= 1")
	}
	width := 15 * cfg.TotalPlayers
	height := 15 * cfg.TotalPlayers

	rand.Seed(time.Now().UnixNano())

	generate := func() MapGrid {
		grid := make(MapGrid, height)
		for y := 0; y < height; y++ {
			grid[y] = make([]TileType, width)
			for x := 0; x < width; x++ {
				if x == 0 || y == 0 || x == width-1 || y == height-1 {
					grid[y][x] = Border
				} else {
					if rand.Float64() < cfg.WalkableProb {
						grid[y][x] = Walkable
					} else {
						grid[y][x] = Obstacle
					}
				}
			}
		}
		return grid
	}

	// Размещает стартовые точки для команд и портал.
	placeSpecialTilesForTeams := func(grid MapGrid, teamsCount int) ([][2]int, [2]int, error) {
		var candidates []struct{ x, y int }
		for y := 1; y < height-1; y++ {
			for x := 1; x < width-1; x++ {
				if grid[y][x] == Walkable {
					candidates = append(candidates, struct{ x, y int }{x, y})
				}
			}
		}
		if len(candidates) < teamsCount+1 {
			return nil, [2]int{}, errors.New("недостаточно проходимых тайлов для размещения специальных точек")
		}
		rand.Shuffle(len(candidates), func(i, j int) {
			candidates[i], candidates[j] = candidates[j], candidates[i]
		})
		var starts [][2]int
		for i := 0; i < teamsCount; i++ {
			pt := candidates[i]
			grid[pt.y][pt.x] = StartTile
			starts = append(starts, [2]int{pt.x, pt.y})
		}
		portalCandidate := candidates[teamsCount]
		grid[portalCandidate.y][portalCandidate.x] = Portal
		portalPos := [2]int{portalCandidate.x, portalCandidate.y}
		return starts, portalPos, nil
	}

	isConnectedFrom := func(grid MapGrid, startX, startY, portalX, portalY int) bool {
		visited := make([][]bool, height)
		for i := range visited {
			visited[i] = make([]bool, width)
		}
		type Point struct{ x, y int }
		queue := []Point{{startX, startY}}
		visited[startY][startX] = true
		dirs := []Point{{0, 1}, {0, -1}, {1, 0}, {-1, 0}}
		for len(queue) > 0 {
			p := queue[0]
			queue = queue[1:]
			if p.x == portalX && p.y == portalY {
				return true
			}
			for _, d := range dirs {
				nx, ny := p.x+d.x, p.y+d.y
				if nx < 0 || nx >= width || ny < 0 || ny >= height {
					continue
				}
				if !visited[ny][nx] && (grid[ny][nx] == Walkable || grid[ny][nx] == StartTile || grid[ny][nx] == Portal) {
					visited[ny][nx] = true
					queue = append(queue, Point{nx, ny})
				}
			}
		}
		return false
	}

	allStartsConnected := func(grid MapGrid, starts [][2]int, portalX, portalY int) bool {
		for _, start := range starts {
			if !isConnectedFrom(grid, start[0], start[1], portalX, portalY) {
				return false
			}
		}
		return true
	}

	placeObjects := func(grid MapGrid) {
		for y := 1; y < height-1; y++ {
			for x := 1; x < width-1; x++ {
				if grid[y][x] != Walkable {
					continue
				}
				r := rand.Float64()
				if r < cfg.MonsterProb {
					grid[y][x] = 'M'
				} else if r < cfg.MonsterProb+cfg.ResourceProb {
					grid[y][x] = 'R'
				}
			}
		}
	}

	var grid MapGrid
	var starts [][2]int
	var portal [2]int

	for {
		grid = generate()
		var err error
		starts, portal, err = placeSpecialTilesForTeams(grid, cfg.TeamsCount)
		if err != nil {
			return nil, nil, [2]int{}, err
		}
		if allStartsConnected(grid, starts, portal[0], portal[1]) {
			break
		}
	}

	placeObjects(grid)
	return grid, starts, portal, nil
}
