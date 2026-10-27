package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math/rand"
	"strings"

	"gameservice/game"
	"gameservice/internal/hexgrid"
	"gameservice/internal/hexmap"
)

// All balance lives on the server. Weights are normalized over level-eligible rewards.
const ProfessionGatherLimit = 10
const ProfessionGatherXP = 250
const ProfessionGatherCellsPerNode = 15

var ErrProfessionGatherGuild = errors.New("Gathering requires the Artisans' Guild at match start.")
var ErrProfessionGatherClass = errors.New("This object requires another gathering profession.")
var ErrProfessionGatherLimit = errors.New("Peaceful gathering limit reached for this match.")

type GatheringMaterial struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	MinLevel    int     `json:"minLevel"`
	Tier        int     `json:"tier"`
	ResourceID  int     `json:"resourceId"`
	Image       string  `json:"image"`
	Chance      float64 `json:"chance"`
	Tradable    bool    `json:"tradable"`
}
type GatheringCategory struct {
	Code           string              `json:"code"`
	Name           string              `json:"name"`
	Description    string              `json:"description"`
	Class          string              `json:"class"`
	Profession     string              `json:"profession"`
	ProfessionName string              `json:"professionName"`
	Image          string              `json:"image"`
	Materials      []GatheringMaterial `json:"materials"`
	Weights        [3]int              `json:"-"`
}

func gatherMaterial(code, name, description string, level, tier int) GatheringMaterial {
	return GatheringMaterial{Code: code, Name: name, Description: description, MinLevel: level, Tier: tier, Image: gatheringMaterialImages[code], Tradable: true}
}

var gatheringCategories = []GatheringCategory{
	{Code: "ore", Weights: [3]int{60, 35, 5}, Name: "Ore veins", Description: "Stone outcrops with metallic veins and faintly glowing minerals.", Class: "guardian", Profession: "miner", ProfessionName: "Miner", Image: "/professions/gathering/gathering_ore_node.png", Materials: []GatheringMaterial{
		gatherMaterial("copper_ore", "Copper ore", "Copper-bearing stone for future metalworking recipes.", 1, 0),
		gatherMaterial("iron_ore", "Iron ore", "Dense iron-bearing ore for future metalworking recipes.", 1, 1),
		gatherMaterial("crystalline_ore", "Crystalline ore", "Rare mineral crystals for future advanced recipes.", 3, 2),
	}},
	{Code: "forest", Weights: [3]int{70, 25, 5}, Name: "Forest deposits", Description: "Old roots, bark and resilient woodland fibers.", Class: "ranger", Profession: "tracker", ProfessionName: "Tracker", Image: "/professions/gathering/gathering_forest_node.png", Materials: []GatheringMaterial{
		gatherMaterial("sturdy_bark", "Sturdy bark", "Tough bark for future protective materials.", 1, 0),
		gatherMaterial("forest_fiber", "Forest fiber", "Flexible woodland fibers for future bindings and fabrics.", 1, 1),
		gatherMaterial("resinous_wood", "Resinous wood", "Rare resin-rich wood for future treated materials.", 3, 2),
	}},
	{Code: "remains", Weights: [3]int{70, 25, 5}, Name: "Creature remains", Description: "Ancient bones and trophies left by forgotten battles.", Class: "berserker", Profession: "trophy_hunter", ProfessionName: "Trophy hunter", Image: "/professions/gathering/gathering_excavation_node.png", Materials: []GatheringMaterial{
		gatherMaterial("bone_fragments", "Bone fragments", "Bone pieces for future carved materials and powders.", 1, 0),
		gatherMaterial("ancient_fang", "Ancient fang", "An ancient trophy for carving weapons and ornaments.", 1, 1),
		gatherMaterial("strong_tendons", "Strong tendons", "Rare durable sinews for bindings and reinforced materials.", 3, 2),
	}},
	{Code: "herbs", Weights: [3]int{75, 20, 5}, Name: "Herbal growths", Description: "Faintly glowing herbs, mushrooms and magical plants.", Class: "mystic", Profession: "herbalist", ProfessionName: "Herbalist", Image: "/professions/gathering/herbs.svg", Materials: []GatheringMaterial{
		gatherMaterial("moon_grass", "Moon grass", "Pale magical grass for future alchemical preparations.", 1, 0),
		gatherMaterial("cave_mushroom", "Cave mushroom", "A cave fungus for future alchemical reagents.", 1, 1),
		gatherMaterial("ethereal_flower", "Ethereal flower", "A rare luminous flower for future advanced alchemy.", 3, 2),
	}},
}

func gatheringCategory(code string) (GatheringCategory, bool) {
	for _, category := range gatheringCategories {
		if category.Code == code {
			return category, true
		}
	}
	return GatheringCategory{}, false
}
func peacefulMaterial(code string) (GatheringMaterial, GatheringCategory, bool) {
	for _, category := range gatheringCategories {
		for _, material := range category.Materials {
			if material.Code == code {
				material.Image = gatheringMaterialImages[material.Code]
				return material, category, true
			}
		}
	}
	return GatheringMaterial{}, GatheringCategory{}, false
}
func GatheringLootCatalog(ctx context.Context) ([]GatheringCategory, error) {
	out := make([]GatheringCategory, 0, len(gatheringCategories))
	for _, category := range gatheringCategories {
		totalWeight := 0
		for _, weight := range category.Weights {
			totalWeight += weight
		}
		category.Materials = append([]GatheringMaterial(nil), category.Materials...)
		for i := range category.Materials {
			material := &category.Materials[i]
			material.Image = gatheringMaterialImages[material.Code]
			material.Chance = float64(category.Weights[material.Tier]) / float64(totalWeight)
			if err := DB.QueryRowContext(ctx, `SELECT id,tradable FROM resources WHERE type=$1 AND resource_kind='resource' ORDER BY id LIMIT 1`, material.Code).Scan(&material.ResourceID, &material.Tradable); err != nil {
				return nil, err
			}
		}
		out = append(out, category)
	}
	return out, nil
}
func ensureGatheringSchemaTx(tx *sql.Tx) error {
	if _, err := tx.Exec(`ALTER TABLE match_players ADD COLUMN IF NOT EXISTS profession_gather_count INTEGER NOT NULL DEFAULT 0 CHECK (profession_gather_count>=0)`); err != nil {
		return err
	}
	for _, category := range gatheringCategories {
		for _, material := range category.Materials {
			if _, err := tx.Exec(`INSERT INTO resources(type,description,effect,image,resource_kind,tradable,market_category,market_subcategory) SELECT $1,$2,'{}',$3,'resource',true,'resource','profession_material' WHERE NOT EXISTS(SELECT 1 FROM resources WHERE type=$1)`, material.Code, material.Description, material.Image); err != nil {
				return err
			}
		}
	}
	return nil
}

// A separate seeded RNG leaves the established dungeon, quest, monster and ordinary
// resource streams unchanged. This runs only when creating a new persisted match.
func placeProfessionNodes(ctx context.Context, snapshot *hexmap.Snapshot, seed int64) error {
	teams := []hexmap.Team{}
	seenTeams := map[int]bool{}
	for _, start := range snapshot.Starts {
		if !seenTeams[start.TeamID] {
			teams = append(teams, hexmap.Team{ID: start.TeamID})
			seenTeams[start.TeamID] = true
		}
	}
	blocked := hexmap.PlacementReservations(*snapshot, hexmap.Config{Teams: teams})
	for _, start := range snapshot.Starts {
		blocked[start.Position] = true
	}
	for _, actor := range snapshot.Actors {
		blocked[actor.Position] = true
	}
	candidates := []int{}
	floors := 0
	for i, cell := range snapshot.Cells {
		if cell.Terrain != "floor" {
			continue
		}
		floors++
		if blocked[cell.Position] || cell.IsPortal || cell.Resource != nil || cell.Barrel != nil || cell.StructureType != "" || cell.EnvironmentType != "" {
			continue
		}
		candidates = append(candidates, i)
	}
	count := floors / ProfessionGatherCellsPerNode
	if count > len(candidates) {
		count = len(candidates)
	}
	if count == 0 {
		return nil
	}
	rng := rand.New(rand.NewSource(seed ^ 0x4755494c44))
	rng.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })
	categories := rng.Perm(len(gatheringCategories))
	// Farthest-point placement distributes the finite count across rooms and corridors.
	chosen := []int{}
	used := map[int]bool{}
	nearest := make(map[int]int, len(candidates))
	for _, index := range candidates {
		nearest[index] = int(^uint(0) >> 1)
	}
	next := candidates[0]
	for len(chosen) < count {
		chosen = append(chosen, next)
		used[next] = true
		best, bestDistance := -1, -1
		for _, index := range candidates {
			if used[index] {
				continue
			}
			d, _ := hexgrid.Distance(snapshot.Cells[index].Position, snapshot.Cells[next].Position)
			if d < nearest[index] {
				nearest[index] = d
			}
			if nearest[index] > bestDistance {
				best, bestDistance = index, nearest[index]
			}
		}
		if best < 0 {
			break
		}
		next = best
	}
	resourceIDs := make(map[string]int, len(gatheringCategories))
	for _, category := range gatheringCategories {
		var id int
		if err := DB.QueryRowContext(ctx, `SELECT id FROM resources WHERE type=$1 AND resource_kind='resource' ORDER BY id LIMIT 1`, category.Materials[0].Code).Scan(&id); err != nil {
			return err
		}
		resourceIDs[category.Code] = id
	}
	for n, index := range chosen {
		category := gatheringCategories[categories[n%len(categories)]]
		snapshot.Cells[index].Resource = &game.ResourceData{ID: resourceIDs[category.Code], Type: category.Name, Description: category.Description, Image: category.Image, ItemType: "resource", ProfessionNode: &game.ProfessionNode{ProfessionName: category.ProfessionName, Category: category.Code, Class: category.Class, Profession: category.Profession}}
	}
	return nil
}

// Called inside the existing collect transaction while match and cell are locked.
// No writes outside this transaction; every failure rolls the original AP debit back.
func (c *MatchCommand) gatherProfessionNodeTx(ctx context.Context, userID int, node *game.ProfessionNode, out *HexCollectResult) error {
	category, ok := gatheringCategory(node.Category)
	if !ok {
		return ErrCommandRejected
	}
	var characterID, count, level int
	var class, profession string
	var unlocked bool
	if err := c.Tx.QueryRowContext(ctx, `SELECT p.character_id,pc.hero_class_id,p.profession_unlocked,p.profession_gather_count,cp.gathering_code,cp.gathering_level FROM match_players p JOIN player_characters pc ON pc.id=p.character_id JOIN character_professions cp ON cp.character_id=p.character_id WHERE p.instance_id=$1 AND p.user_id=$2 FOR UPDATE OF p`, c.InstanceID, userID).Scan(&characterID, &class, &unlocked, &count, &profession, &level); err != nil {
		return err
	}
	if !unlocked {
		return ErrProfessionGatherGuild
	}
	if class != category.Class || profession != category.Profession {
		return ErrProfessionGatherClass
	}
	if count >= ProfessionGatherLimit {
		return ErrProfessionGatherLimit
	}
	total := 0
	for _, m := range category.Materials {
		if m.MinLevel <= level {
			total += category.Weights[m.Tier]
		}
	}
	if total <= 0 {
		return ErrCommandRejected
	}
	roll := rand.Intn(total)
	var reward GatheringMaterial
	for _, m := range category.Materials {
		if m.MinLevel > level {
			continue
		}
		roll -= category.Weights[m.Tier]
		if roll < 0 {
			reward = m
			break
		}
	}
	if reward.Code == "" {
		return ErrCommandRejected
	}
	var image, description string
	if err := c.Tx.QueryRowContext(ctx, `SELECT id,image,description FROM resources WHERE type=$1 AND resource_kind='resource' ORDER BY id LIMIT 1`, reward.Code).Scan(&out.ItemID, &image, &description); err != nil {
		return err
	}
	updated, err := AddInventoryItemTx(c.Tx, c.InstanceID, userID, "resource", out.ItemID, reward.Name, image, description, 1)
	if err != nil {
		return err
	}
	if _, err = c.Tx.ExecContext(ctx, `UPDATE match_players SET profession_gather_count=profession_gather_count+1 WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, userID); err != nil {
		return err
	}
	awardedXP := 0
	if level < ProfessionMaxLevel {
		if _, err = c.Tx.ExecContext(ctx, `INSERT INTO match_profession_xp(match_instance_id,character_id,profession_code,xp) VALUES($1,$2,$3,$4) ON CONFLICT(match_instance_id,character_id,profession_code) DO UPDATE SET xp=match_profession_xp.xp+EXCLUDED.xp`, c.InstanceID, characterID, profession, ProfessionGatherXP); err != nil {
			return err
		}
		awardedXP = ProfessionGatherXP
	}

	out.Inventory = json.RawMessage(updated)
	out.ItemType = "resource"
	out.ItemName = reward.Name
	out.ProfessionGathering = true
	out.ProfessionXP = awardedXP
	return nil
}

// Metadata helpers feed the existing inventory, market and extraction paths.
func peacefulOrigin(code string) string {
	_, category, ok := peacefulMaterial(code)
	if !ok {
		return ""
	}
	return category.Name
}
func peacefulResourceNameCases() string {
	s := ""
	for _, c := range gatheringCategories {
		for _, m := range c.Materials {
			s += " WHEN '" + m.Code + "' THEN '" + strings.ReplaceAll(m.Name, "'", "''") + "'"
		}
	}
	return s
}
func peacefulResourceTypes() []string {
	out := []string{}
	for _, c := range gatheringCategories {
		for _, m := range c.Materials {
			out = append(out, "'"+m.Code+"'")
		}
	}
	return out
}
