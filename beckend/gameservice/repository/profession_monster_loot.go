package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"strings"
)

const PROFESSION_MONSTER_DROP_CHANCE = 0.05
const ProfessionMonsterDropLimit = 10

type ProfessionMaterial struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Image       string `json:"image"`
	Class       string `json:"class"`
	Processing  string `json:"processing"`
}

var professionMaterials = []ProfessionMaterial{
	{"goblin_tooth", "Зуб гоблина", "Небольшой крепкий зуб с необычайно плотной эмалью. Перетирается в порошок для алхимии и магических чернил.", "/professions/monster-loot/goblin/goblin_tooth.png", "berserker", "Костяной порошок"},
	{"goblin_tendon", "Сухожилие гоблина", "Высушенные эластичные волокна, сохраняющие прочность. Используются для изготовления нитей, креплений и тетив.", "/professions/monster-loot/goblin/goblin_tendon.png", "ranger", "Нити, крепления и тетивы"},
	{"goblin_claw", "Коготь гоблина", "Тёмный изогнутый коготь, твёрдый словно камень. Используется для изготовления шипов и оружейных деталей.", "/professions/monster-loot/goblin/goblin_claw.png", "guardian", "Шипы и оружейные детали"},
	{"goblin_earwax", "Ушная сера гоблина", "Липкий алхимический реагент янтарного цвета. Используется в связующих смесях и магических чернилах.", "/professions/monster-loot/goblin/goblin_earwax.png", "mystic", "Связующие смеси и чернила"},
	{"orc_tusk", "Клык орка", "Массивный клык с плотной костной структурой. Подходит для изготовления порошков, наконечников и деталей оружия.", "/professions/monster-loot/orc/orc_tusk.png", "berserker", "Порошки и наконечники"},
	{"orc_hide", "Кожа орка", "Грубая плотная кожа, устойчивая к повреждениям. Используется кожевниками для создания прочной экипировки.", "/professions/monster-loot/orc/orc_hide.png", "ranger", "Прочная кожа"},
	{"orc_bone_plate", "Костяная пластина орка", "Прочная костяная пластина с природными укреплениями. Используется для усиления тяжёлой брони и щитов.", "/professions/monster-loot/orc/orc_bone_plate.png", "guardian", "Усиление брони и щитов"},
	{"orc_toxin", "Токсин орка", "Ядовитый зеленоватый концентрат. Ценный реагент для алхимических составов и ядов.", "/professions/monster-loot/orc/orc_toxin.png", "mystic", "Алхимические составы и яды"},
	{"troll_tooth", "Зуб тролля", "Огромный зуб исключительной плотности. Используется для производства редкого костяного порошка.", "/professions/monster-loot/troll/troll_tooth.png", "berserker", "Редкий костяной порошок"},
	{"troll_hide", "Шкура тролля", "Толстая шкура с природными защитными чешуйками. Используется для изготовления усиленной кожи.", "/professions/monster-loot/troll/troll_hide.png", "ranger", "Усиленная кожа"},
	{"troll_horn_plate", "Роговая пластина тролля", "Массивная прочная пластина, почти не уступающая металлу. Применяется для укрепления тяжёлой экипировки.", "/professions/monster-loot/troll/troll_horn_plate.png", "guardian", "Укрепление тяжёлой экипировки"},
	{"troll_regenerative_gland", "Регенеративная железа тролля", "Необычный органический сгусток, сохраняющий магическую энергию восстановления. Используется в редких алхимических составах.", "/professions/monster-loot/troll/troll_regenerative_gland.png", "mystic", "Редкие алхимические составы"},
}

type professionMonster struct {
	Code, Name, Family string
	Quantity, XP       int
}

var professionMonsters = []professionMonster{
	{"Goblin", "Гоблин", "goblin", 1, 2000},
	{"Goblins", "Два гоблина", "goblin", 2, 2500},
	{"Orc", "Орк", "orc", 1, 3000},
	{"Troll", "Тролль", "troll", 1, 3500},
}

func professionMaterial(code string) (ProfessionMaterial, bool) {
	for _, m := range professionMaterials {
		if m.Code == code {
			return m, true
		}
	}
	return ProfessionMaterial{}, false
}

// Shared by all market queries, including search, without changing stable resource types.
var professionResourceNameSQL = func() string {
	s := "CASE r.type"
	for _, m := range professionMaterials {
		s += " WHEN '" + m.Code + "' THEN '" + strings.ReplaceAll(m.Name, "'", "''") + "'"
	}
	return s + peacefulResourceNameCases() + craftResourceNameCases() + " ELSE r.type END"
}()

// These materials never enter existing map-cell or barrel pools.
var professionMaterialTypesSQL = func() string {
	codes := make([]string, 0, len(professionMaterials))
	for _, m := range professionMaterials {
		codes = append(codes, "'"+strings.ReplaceAll(m.Code, "'", "''")+"'")
	}
	return strings.Join(append(append(codes, peacefulResourceTypes()...), craftResourceTypes()...), ",")
}()

// Called by the existing startup schema mechanism, after resources/market schema.
func EnsureProfessionMonsterLootSchema() error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serializes concurrent application startup without imposing uniqueness on legacy resources.
	if _, err = tx.Exec(`LOCK TABLE resources IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return err
	}
	// Older seeds explicitly used resource IDs. Advance only, never rewrite existing IDs.
	if _, err = tx.Exec(`SELECT setval(pg_get_serial_sequence('resources','id'),GREATEST(COALESCE((SELECT MAX(id) FROM resources),1),(SELECT last_value FROM resources_id_seq)),true)`); err != nil {
		return err
	}
	for _, m := range professionMaterials {
		if _, err = tx.Exec(`INSERT INTO resources(type,description,effect,image,resource_kind,tradable,market_category,market_subcategory)
   SELECT $1,$2,'{}',$3,'resource',true,'resource','profession_material'
   WHERE NOT EXISTS(SELECT 1 FROM resources WHERE type=$1)`, m.Code, m.Description, m.Image); err != nil {
			return err
		}
	}
	_, err = tx.Exec(`ALTER TABLE match_players ADD COLUMN IF NOT EXISTS profession_unlocked BOOLEAN NOT NULL DEFAULT false;
 CREATE TABLE IF NOT EXISTS match_profession_monster_drops (
  match_instance_id TEXT NOT NULL REFERENCES matches(instance_id) ON DELETE CASCADE,
  monster_instance_id INTEGER NOT NULL,
  character_id INTEGER NOT NULL REFERENCES player_characters(id) ON DELETE CASCADE,
  success BOOLEAN NOT NULL,
  PRIMARY KEY(match_instance_id,monster_instance_id)
 );
 CREATE INDEX IF NOT EXISTS match_profession_drop_success ON match_profession_monster_drops(match_instance_id,character_id) WHERE success`)
	if err != nil {
		return err
	}
	if err := ensureGatheringSchemaTx(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// The caller holds the match row lock and has confirmed lethal damage. Run before
// deleting the monster; its catalog identity must still be available. Every outcome
// (including failed/capped rolls) is durable in the same kill transaction.
func (c *MatchCommand) GrantProfessionMonsterLootTx(ctx context.Context, userID, monsterID int) error {
	var characterID int
	var class, monsterCode string
	err := c.Tx.QueryRowContext(ctx, `SELECT p.character_id,pc.hero_class_id,m.name
 FROM match_players p JOIN player_characters pc ON pc.id=p.character_id
 JOIN match_monsters mm ON mm.instance_id=p.instance_id AND mm.monster_instance_id=$3
 JOIN monsters m ON m.id=mm.monster_ref_id
 JOIN matches live ON live.instance_id=p.instance_id
 WHERE p.instance_id=$1 AND p.user_id=$2 AND p.health>0 AND mm.health>0
 AND p.profession_unlocked=true
 AND live.geometry->>'kind'='hex' AND live.schema_version=2 AND live.mode<>'hex-test' AND live.finished_at IS NULL
 AND NOT EXISTS(SELECT 1 FROM hex_player_results r WHERE r.instance_id=p.instance_id AND r.user_id=p.user_id)
 FOR UPDATE OF p`, c.InstanceID, userID, monsterID).Scan(&characterID, &class, &monsterCode)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var monster *professionMonster
	for i := range professionMonsters {
		if professionMonsters[i].Code == monsterCode {
			monster = &professionMonsters[i]
			break
		}
	}
	if monster == nil {
		return nil
	}
	var material *ProfessionMaterial
	for i := range professionMaterials {
		m := &professionMaterials[i]
		if m.Class == class && strings.HasPrefix(m.Code, monster.Family+"_") {
			material = m
			break
		}
	}
	if material == nil {
		return nil
	}
	var alreadyRolled bool
	if err = c.Tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM match_profession_monster_drops WHERE match_instance_id=$1 AND monster_instance_id=$2)`, c.InstanceID, monsterID).Scan(&alreadyRolled); err != nil {
		return err
	}
	if alreadyRolled {
		return nil
	}
	var successes int
	if err = c.Tx.QueryRowContext(ctx, `SELECT count(*) FROM match_profession_monster_drops WHERE match_instance_id=$1 AND character_id=$2 AND success`, c.InstanceID, characterID).Scan(&successes); err != nil {
		return err
	}
	success := successes < ProfessionMonsterDropLimit && rand.Float64() < PROFESSION_MONSTER_DROP_CHANCE
	result, err := c.Tx.ExecContext(ctx, `INSERT INTO match_profession_monster_drops(match_instance_id,monster_instance_id,character_id,success) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, c.InstanceID, monsterID, characterID, success)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 || !success {
		return nil
	}
	var id int
	var image, description string
	if err = c.Tx.QueryRowContext(ctx, `SELECT id,image,description FROM resources WHERE type=$1 AND resource_kind='resource' ORDER BY id LIMIT 1`, material.Code).Scan(&id, &image, &description); err != nil {
		return err
	}
	if _, err = AddInventoryItemTx(c.Tx, c.InstanceID, userID, "resource", id, material.Name, image, description, monster.Quantity); err != nil {
		return err
	}
	profession, err := gatheringProfessionForClass(class)
	if err != nil {
		return err
	}
	_, err = c.Tx.ExecContext(ctx, `INSERT INTO match_profession_xp(match_instance_id,character_id,profession_code,xp) SELECT $1,$2,$3,$4 WHERE EXISTS(SELECT 1 FROM character_professions WHERE character_id=$2 AND gathering_level<$5) ON CONFLICT(match_instance_id,character_id,profession_code) DO UPDATE SET xp=match_profession_xp.xp+EXCLUDED.xp`, c.InstanceID, characterID, profession, monster.XP, ProfessionMaxLevel)
	return err
}

func transferHexProfessionXPTx(ctx context.Context, tx *sql.Tx, instanceID string, characterID int) error {
	// Eligibility is the creation-time snapshot, never the current base level.
	// Older matches default to false; retain legacy staged XP without awarding it.
	var unlocked bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM match_players WHERE instance_id=$1 AND character_id=$2 AND profession_unlocked=true)`, instanceID, characterID).Scan(&unlocked); err != nil {
		return err
	}
	if !unlocked {
		return nil
	}

	var class string
	if err := tx.QueryRowContext(ctx, `SELECT hero_class_id FROM player_characters WHERE id=$1`, characterID).Scan(&class); err != nil {
		return err
	}
	if err := ensureCharacterProfessionTx(tx, characterID, class); err != nil {
		return err
	}
	var code string
	var level int
	var xp, awarded int64
	if err := tx.QueryRowContext(ctx, `SELECT gathering_code,gathering_level,gathering_xp FROM character_professions WHERE character_id=$1 FOR UPDATE`, characterID).Scan(&code, &level, &xp); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(sum(xp),0) FROM match_profession_xp WHERE match_instance_id=$1 AND character_id=$2 AND profession_code=$3`, instanceID, characterID, code).Scan(&awarded); err != nil {
		return err
	}
	level, xp, err := AdvanceProfessionXP(level, xp, awarded)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE character_professions SET gathering_level=$2,gathering_xp=$3,updated_at=now() WHERE character_id=$1`, characterID, level, xp); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM match_profession_xp WHERE match_instance_id=$1 AND character_id=$2`, instanceID, characterID)
	return err
}

func professionMaterialOrigin(code string) string {
	family := strings.SplitN(code, "_", 2)[0]
	switch family {
	case "goblin":
		return "Гоблин / Два гоблина"
	case "orc":
		return "Орк"
	case "troll":
		return "Тролль"
	}
	return ""
}

type ProfessionLootEntry struct {
	ProfessionMaterial
	ResourceID     int     `json:"resourceId"`
	ProfessionCode string  `json:"professionCode"`
	ProfessionName string  `json:"professionName"`
	Quantity       int     `json:"quantity"`
	Chance         float64 `json:"chance"`
	Tradable       bool    `json:"tradable"`
}
type ProfessionLootMonster struct {
	Code             string                `json:"code"`
	Name             string                `json:"name"`
	Image            string                `json:"image"`
	Health           int                   `json:"health"`
	Attack           int                   `json:"attack"`
	Defense          int                   `json:"defense"`
	XP               int                   `json:"xp"`
	ProfessionChance float64               `json:"professionChance"`
	EquipmentChance  float64               `json:"equipmentChance"`
	Materials        []ProfessionLootEntry `json:"materials"`
}

func ProfessionLootCatalog(ctx context.Context) ([]ProfessionLootMonster, error) {
	out := []ProfessionLootMonster{}
	tx, err := DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	equipmentChance := float64(EquipmentDropChancePercent()) / 100
	for _, monster := range professionMonsters {
		entry := ProfessionLootMonster{Code: monster.Code, Name: monster.Name, XP: monster.XP, ProfessionChance: PROFESSION_MONSTER_DROP_CHANCE, EquipmentChance: equipmentChance, Materials: []ProfessionLootEntry{}}
		if err := tx.QueryRowContext(ctx, `SELECT image,max_health,attack,defense FROM monsters WHERE name=$1 ORDER BY id LIMIT 1`, monster.Code).Scan(&entry.Image, &entry.Health, &entry.Attack, &entry.Defense); err != nil {
			return nil, fmt.Errorf("loot monster %s: %w", monster.Code, err)
		}
		entry.Image = strings.Trim(entry.Image, "\"")
		for _, m := range professionMaterials {
			if !strings.HasPrefix(m.Code, monster.Family+"_") {
				continue
			}
			loot := ProfessionLootEntry{ProfessionMaterial: m, Quantity: monster.Quantity, Chance: PROFESSION_MONSTER_DROP_CHANCE}
			loot.ProfessionCode, err = gatheringProfessionForClass(m.Class)
			if err != nil {
				return nil, err
			}
			if err := tx.QueryRowContext(ctx, `SELECT r.id,r.description,r.image,r.tradable,p.name FROM resources r CROSS JOIN profession_catalog p WHERE r.type=$1 AND r.resource_kind='resource' AND p.code=$2 ORDER BY r.id LIMIT 1`, m.Code, loot.ProfessionCode).Scan(&loot.ResourceID, &loot.Description, &loot.Image, &loot.Tradable, &loot.ProfessionName); err != nil {
				return nil, err
			}
			entry.Materials = append(entry.Materials, loot)
		}
		out = append(out, entry)
	}
	return out, tx.Commit()
}

// Storage and market delivery share the existing JSON inventory. Refresh only
// professional material labels and origins; do not reinterpret legacy items.
func enrichProfessionStorageItems(ctx context.Context, query storageQuerier, items map[string]map[string]any) error {
	for key, item := range items {
		if !strings.HasPrefix(key, "resource_") {
			continue
		}
		var code string
		err := query.QueryRowContext(ctx, `SELECT type FROM resources WHERE id::text=$1 AND resource_kind='resource'`, strings.TrimPrefix(key, "resource_")).Scan(&code)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if recipe, ok := craftRecipe(code); ok {
			item["name"], item["item_name"], item["origin"], item["resource_code"] = recipe.Name, recipe.Name, "Artisans' Guild", code
			// Stored stacks can still carry the placeholder used before powder PNGs
			// arrived. Resolve it from the same catalogue without rewriting storage.
			if code == "copper_dust" || code == "iron_filings" || code == "crystal_dust" {
				if image, _ := item["image"].(string); image == "" || image == "/ui-icons/backpack.png" {
					item["image"] = recipe.Image
				}
			}
		}
		if material, _, ok := peacefulMaterial(code); ok {
			item["name"], item["item_name"], item["origin"], item["resource_code"] = material.Name, material.Name, peacefulOrigin(code), code
			continue
		}
		m, ok := professionMaterial(code)
		if !ok {
			continue
		}
		item["name"], item["item_name"], item["origin"], item["resource_code"] = m.Name, m.Name, professionMaterialOrigin(code), code
	}
	return nil
}
