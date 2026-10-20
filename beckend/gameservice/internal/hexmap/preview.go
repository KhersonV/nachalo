// Package hexmap builds deterministic pointy-top HEX dungeon plans without SQL.
package hexmap

import (
	"gameservice/game"
	"gameservice/internal/hexgrid"
	"gameservice/models"
)

type Cell struct {
	CellID   int                `json:"cellId"`
	Position hexgrid.Coord      `json:"position"`
	Terrain  string             `json:"terrain"`
	Room     int                `json:"room,omitempty"`
	Resource *game.ResourceData `json:"resource,omitempty"`
	Barrel   *game.ResourceData `json:"barrel,omitempty"`
	IsPortal bool               `json:"isPortal,omitempty"`
}
type Edge struct {
	Position  hexgrid.Coord `json:"position"`
	Direction int           `json:"direction"`
	Kind      string        `json:"kind"`
}
type Actor struct {
	ID              string        `json:"id"`
	Kind            string        `json:"kind"`
	Position        hexgrid.Coord `json:"position"`
	Health          int           `json:"health,omitempty"`
	MaxHealth       int           `json:"maxHealth,omitempty"`
	MonsterRefID    int           `json:"monsterRefId,omitempty"`
	Name            string        `json:"name,omitempty"`
	Type            string        `json:"type,omitempty"`
	Image           string        `json:"image,omitempty"`
	Attack          int           `json:"attack,omitempty"`
	Defense         int           `json:"defense,omitempty"`
	Speed           int           `json:"speed,omitempty"`
	Maneuverability int           `json:"maneuverability,omitempty"`
	Vision          int           `json:"vision,omitempty"`
}
type Player struct {
	UserID        int            `json:"userId"`
	Name          string         `json:"name"`
	Position      hexgrid.Coord  `json:"position"`
	Energy        int            `json:"energy"`
	MaxEnergy     int            `json:"maxEnergy"`
	Health        int            `json:"health"`
	MaxHealth     int            `json:"maxHealth"`
	Mobility      int            `json:"mobility"`
	Agility       int            `json:"agility"`
	CharacterType string         `json:"characterType"`
	IsRanged      bool           `json:"isRanged"`
	AttackRange   int            `json:"attackRange"`
	Inventory     map[string]any `json:"inventory,omitempty"`
}
type Snapshot struct {
	SchemaVersion int             `json:"schemaVersion"`
	Geometry      models.Geometry `json:"geometry"`
	InstanceID    string          `json:"instanceId"`
	MapRevision   int64           `json:"mapRevision"`
	StateRevision int64           `json:"stateRevision"`
	Seed          int64           `json:"seed"`
	Radius        int             `json:"radius"`
	Starts        []Start         `json:"starts,omitempty"`
	Portal        *hexgrid.Coord  `json:"portal,omitempty"`
	Cells         []Cell          `json:"cells"`
	Edges         []Edge          `json:"edges"`
	Actors        []Actor         `json:"actors"`
	Player        *Player         `json:"player,omitempty"`
	ActiveUser    int             `json:"activeUser,omitempty"`
	TurnNumber    int             `json:"turnNumber,omitempty"`
}

// Generate keeps the read-only preview endpoint. Its illustrative catalog is
// local to the preview; playable matches pass PostgreSQL catalogs to GenerateDungeon.
func Generate(seed int64) (Snapshot, error) {
	return GenerateDungeon(Config{Teams: []Team{{ID: 1, PlayerIDs: []int{1}}}, MonsterProb: 0.05, BarrelProb: 0.1, ResourceProb: 0.1}, Catalogs{
		Monsters:  []game.MonsterData{{ID: 1, Name: "Preview monster", Health: 35, MaxHealth: 35, Attack: 8, Defense: 3}},
		Resources: []game.ResourceData{{ID: 1, Type: "food"}, {ID: 6, Type: "barrel"}},
	}, seed)
}
