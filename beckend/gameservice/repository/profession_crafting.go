package repository

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

//go:embed profession_crafting_catalog.json
var craftingCatalogJSON []byte

//go:embed sql/professions_stage_10_3.sql
var craftingSchemaSQL string

var ErrCraftInvalid = errors.New("invalid crafting request")
var ErrCraftIngredients = errors.New("not enough crafting ingredients")
var ErrCraftLevel = errors.New("processing profession level too low")
var ErrCraftProfession = errors.New("another processing profession is required")
var ErrCraftReplay = errors.New("This crafting request was already used for another item.")

type CraftIngredient struct {
	Code       string `json:"code"`
	Quantity   int    `json:"quantity"`
	ResourceID int    `json:"resourceId"`
	Name       string `json:"name"`
	Image      string `json:"image"`
	Owned      int    `json:"owned"`
}
type CraftRecipe struct {
 Rarity string `json:"rarity,omitempty"`
 JewelrySet *JewelryPair `json:"jewelrySet,omitempty"`
 JewelryProgress int `json:"jewelryProgress"`
	Code         string            `json:"code"`
	Name         string            `json:"name"`
	RU           string            `json:"ru"`
	DE           string            `json:"de"`
	Profession   string            `json:"profession"`
	MinLevel     int               `json:"minLevel"`
	Ingredients  []CraftIngredient `json:"ingredients"`
	Output       int               `json:"output"`
	XP           int               `json:"xp"`
	Image        string            `json:"image"`
	Category     string            `json:"category"`
	Effect       string            `json:"effect"`
	PocketStack  int               `json:"pocketStack"`
	BaseTemplate string            `json:"baseTemplate,omitempty"`
	Slot         string            `json:"slot,omitempty"`
	HeroClass    string            `json:"heroClass,omitempty"`
	ResultID     int64             `json:"resultId"`
	Available    bool              `json:"available"`
	Reason       string            `json:"reason,omitempty"`
	Description  string            `json:"description"`
	Bonuses      EquipmentBonuses  `json:"bonuses"`
	Durability   int               `json:"durability,omitempty"`
	PocketBonus  int               `json:"pocketBonus,omitempty"`
	Tradable     bool              `json:"tradable"`
}

var craftingRecipes = func() []CraftRecipe {
	var out []CraftRecipe
	if err := json.Unmarshal(craftingCatalogJSON, &out); err != nil {
		panic(err)
	}
	out = append(out, jewelryRecipes()...)
	for i := range out {
		r := &out[i]
		if r.Category == "equipment" {
 r.Rarity = "blue"
			item, set, ok := equipmentCatalogItem(r.Code)
			if !ok || set.Source != "craft" || set.Rarity != "blue" || set.HeroClass != r.HeroClass || item.Slot != r.Slot || item.BaseTemplate != r.BaseTemplate {
				panic("invalid equipment craft catalog: " + r.Code)
			}
			r.Name, r.Image = item.Name, item.Image
		}
	}
	return out
}()

func craftRecipe(code string) (CraftRecipe, bool) {
	for _, r := range craftingRecipes {
		if r.Code == code {
			return r, true
		}
	}
	return CraftRecipe{}, false
}
func craftResourceNameCases() string {
	s := ""
	for _, r := range craftingRecipes {
		if r.Category != "equipment" && r.Category != "jewelry" {
			s += " WHEN '" + r.Code + "' THEN '" + strings.ReplaceAll(r.Name, "'", "''") + "'"
		}
	}
 for _, r := range jewelryCatalog.Resources { s += " WHEN '" + r.Code + "' THEN '" + strings.ReplaceAll(r.Name,"'","''") + "'" }
 return s
}
func craftResourceTypes() []string {
	out := []string{}
	for _, r := range craftingRecipes {
		if r.Category != "equipment" && r.Category != "jewelry" {
			out = append(out, "'"+r.Code+"'")
		}
	}
 for _, r := range jewelryCatalog.Resources { out = append(out,"'"+r.Code+"'") }
 return out
}

func EnsureCraftingSchema() error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`LOCK TABLE resources IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return err
	}
	if _, err = tx.Exec(jewelrySchemaSQL); err != nil { return err }
	if _, err = tx.Exec(craftingSchemaSQL); err != nil {
		return err
	}
	if _, err = tx.Exec(`SELECT setval(pg_get_serial_sequence('resources','id'),GREATEST(COALESCE((SELECT MAX(id) FROM resources),1),(SELECT last_value FROM resources_id_seq)),true)`); err != nil {
		return err
	}
	for _, r := range craftingRecipes {
 if r.Category == "jewelry" { continue }
		if r.Category == "equipment" {
			// Derive a compatible existing class/slot template, keeping gray items intact.
			_, err = tx.Exec(`INSERT INTO item_templates(code,name,slot,item_type,handedness,rarity,class_restriction,image_url,attack_bonus,defense_bonus,mobility_bonus,agility_bonus,max_health_bonus,sight_range_bonus,attack_range_bonus,pocket_bonus,tradable,npc_price,sell_price)
    SELECT $1,$2,slot,item_type,handedness,'blue',class_restriction,$3,attack_bonus+CASE WHEN slot='main_hand' THEN 2 ELSE 0 END,defense_bonus+CASE WHEN slot<>'main_hand' THEN 2 ELSE 0 END,mobility_bonus,agility_bonus,max_health_bonus+4,sight_range_bonus,attack_range_bonus,CASE WHEN slot IN ('chest','pants') THEN 2 ELSE 0 END,true,0,0 FROM item_templates WHERE code=$4 ON CONFLICT(code) DO NOTHING`, r.Code, r.Name, r.Image, r.BaseTemplate)
		} else {
			effect := "{}"
			if r.Effect != "" {
				effect = `{"craft_consumable":1}`
			}
			_, err = tx.Exec(`INSERT INTO resources(type,description,effect,image,resource_kind,tradable,market_category,market_subcategory) SELECT $1,$2,$3,$4,'resource',true,'resource',$5 WHERE NOT EXISTS(SELECT 1 FROM resources WHERE type=$1)`, r.Code, craftEffectDescription(r), effect, r.Image, r.Category)
		}
		if err != nil {
			return fmt.Errorf("craft catalog %s: %w", r.Code, err)
		}
		// Stage 10.4 installations may already have these resources with the
		// neutral fallback. Upgrade only that placeholder, retaining IDs/custom art.
		if r.Code == "copper_dust" || r.Code == "iron_filings" || r.Code == "crystal_dust" {
			if _, err = tx.Exec(`UPDATE resources SET image=$2 WHERE type=$1 AND (image IS NULL OR image='' OR image='/ui-icons/backpack.png')`, r.Code, r.Image); err != nil {
				return err
			}
		}
	}
	// These two supplied PNGs were renamed to their canonical spellings. Only
	// replace the old missing paths; retain custom art and existing resource IDs.
	for _, legacy := range []struct{ code, image string }{
		{"sharpened_fangs", "/professions/processed/ craft_sharpened_fangs.png"},
		{"speed_potion", "/professions/consumables/craft_speed_potionа.png"},
	} {
		r, _ := craftRecipe(legacy.code)
		if _, err = tx.Exec(`UPDATE resources SET image=$3 WHERE type=$1 AND image=$2`, legacy.code, legacy.image, r.Image); err != nil {
			return err
		}
	}
	// Connect real material art without changing gathering rules or custom art.
	for code, image := range gatheringMaterialImages {
		if _, err = tx.Exec(`UPDATE resources SET image=$2 WHERE type=$1 AND image IN ('/professions/gathering/ore.svg','/professions/gathering/forest.svg','/professions/gathering/remains.svg','/professions/gathering/herbs.svg')`, code, image); err != nil {
			return err
		}
	}
	if err = seedEquipmentSetsTx(tx); err != nil {
		return err
	}
	return tx.Commit()
}
func craftResourceTx(ctx context.Context, q storageQuerier, code string) (int, string, string, string, error) {
	var id int
	var image, description, effect string
	err := q.QueryRowContext(ctx, `SELECT id,COALESCE(image,''),COALESCE(description,''),COALESCE(effect,'{}') FROM resources WHERE type=$1 AND resource_kind='resource' ORDER BY id LIMIT 1`, code).Scan(&id, &image, &description, &effect)
	return id, image, description, effect, err
}
func resourceDisplayName(code string) string {
 if r,ok:=JewelryResourceByCode(code);ok { return r.Name }
	if name, ok := map[string]string{"water": "Water", "food": "Food", "wood": "Wood", "stone": "Stone", "iron": "Iron"}[code]; ok {
		return name
	}
	if r, ok := craftRecipe(code); ok {
		return r.Name
	}
	if m, _, ok := peacefulMaterial(code); ok {
		return m.Name
	}
	if m, ok := professionMaterial(code); ok {
		return m.Name
	}
	return code
}
func craftOutputEntry(r CraftRecipe, id int, image, description, effect string, count int) (map[string]any, error) {
	var effects map[string]int
	if err := json.Unmarshal([]byte(effect), &effects); err != nil {
		return nil, err
	}
	return map[string]any{"item_type": "resource", "item_id": id, "resource_code": r.Code, "name": r.Name, "item_name": r.Name, "image": image, "description": description, "effect": effects, "item_count": count, "origin": "Artisans' Guild", "consumable_effect": r.Effect, "consumable_kind": r.Category, "pocket_stack": r.PocketStack}, nil
}
func enrichCraftRecipes(ctx context.Context, q storageQuerier, inventory map[string]map[string]any, prof string, level int, guild bool) ([]CraftRecipe, error) {
	out := make([]CraftRecipe, 0, len(craftingRecipes))
	for _, r := range craftingRecipes {
		r.Ingredients = append([]CraftIngredient(nil), r.Ingredients...)
		r.Available = true
		r.Description = craftEffectDescription(r)
 if r.Category == "jewelry" { r.Description = "Craft-only jewelry. Ring and amulet of the same series grant a 2/2 bonus." }
		for i := range r.Ingredients {
			in := &r.Ingredients[i]
			id, image, _, _, err := craftResourceTx(ctx, q, in.Code)
			if err != nil {
				return nil, err
			}
			in.ResourceID, in.Image, in.Name, in.Owned = id, image, resourceDisplayName(in.Code), storageCount(inventory[fmt.Sprintf("resource_%d", id)])
			if in.Owned < in.Quantity {
				r.Available = false
				r.Reason = "not enough crafting ingredients"
			}
		}
		var err error
		if r.Category == "equipment" || r.Category == "jewelry" {
			r.Durability = 150
			err = q.QueryRowContext(ctx, `SELECT id,image_url,attack_bonus,defense_bonus,mobility_bonus,agility_bonus,max_health_bonus,`+equipmentPocketBonusSQL+`,tradable FROM item_templates it WHERE code=$1 AND rarity=$2`, r.Code, r.Rarity).Scan(&r.ResultID, &r.Image, &r.Bonuses.Attack, &r.Bonuses.Defense, &r.Bonuses.Mobility, &r.Bonuses.Agility, &r.Bonuses.MaxHealth, &r.PocketBonus, &r.Tradable)
			r.Bonuses.PocketBonus = r.PocketBonus
		} else {
			err = q.QueryRowContext(ctx, `SELECT id,tradable FROM resources WHERE type=$1 AND resource_kind='resource' ORDER BY id LIMIT 1`, r.Code).Scan(&r.ResultID, &r.Tradable)
		}
		if err != nil {
			return nil, err
		}
		if level < r.MinLevel {
			r.Available = false
			r.Reason = "processing profession level too low"
		}
		if prof != r.Profession {
			r.Available = false
			r.Reason = "another processing profession is required"
		}
		if !guild {
			r.Available = false
			r.Reason = "guild_required"
		}
		out = append(out, r)
	}
	return out, nil
}
func GetCraftRecipes(ctx context.Context, user, character int) ([]CraftRecipe, error) {
	state, err := GetCharacterProfessions(ctx, user, character)
	if err != nil {
		return nil, err
	}
	var raw []byte
	if err = DB.QueryRowContext(ctx, `SELECT inventory FROM player_profiles WHERE user_id=$1`, user).Scan(&raw); err != nil {
		return nil, err
	}
	inv, err := parseStorage(raw)
	if err != nil {
		return nil, err
	}
	prof, level := "", 1
	if state.Processing != nil {
		prof, level = state.Processing.Code, state.Processing.Level
	}
	recipes, err := enrichCraftRecipes(ctx, DB, inv, prof, level, state.Guild.Built)
 if err != nil { return nil, err }
 equipped, err := GetCharacterEquipment(state.CharacterID)
 if err != nil { return nil, err }
 for i := range recipes {
  r := &recipes[i]
  if _,set,ok := jewelryItem(r.Code); ok && set.HeroClass == state.HeroClassID {
   for _,slot := range []string{"ring","amulet"} { if item:=equipped[slot]; item!=nil && item.SetCode==set.Code && item.ClassRestriction==set.HeroClass {r.JewelryProgress++} }
  }
 }
 return recipes, nil
}

// Public links contain no account inventory or profession progress.
func PublicCraftRecipes(ctx context.Context) ([]CraftRecipe, error) {
	return enrichCraftRecipes(ctx, DB, map[string]map[string]any{}, "", 1, false)
}

type CraftResult struct {
	ProgressionVersion int                       `json:"progressionVersion"`
	ItemInstances      []string                  `json:"itemInstances,omitempty"`
	CharacterID        int                       `json:"characterId"`
	RecipeCode         string                    `json:"recipeCode"`
	Quantity           int                       `json:"quantity"`
	ResultQuantity     int                       `json:"resultQuantity"`
	Level              int                       `json:"level"`
	XP                 int64                     `json:"xp"`
	Inventory          map[string]map[string]any `json:"inventory"`
}

func CraftProfessionItem(ctx context.Context, user, character int, code string, quantity int, command string) (CraftResult, error) {
	var out CraftResult
	r, ok := craftRecipe(code)
	if !ok || character <= 0 || quantity < 1 || quantity > 100 || len(command) < 8 || len(command) > 128 {
		return out, ErrCraftInvalid
	}
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	// Shared inventory lock serializes crafting, marketplace and match loadout transfers.
	var raw []byte
	if err = tx.QueryRowContext(ctx, `SELECT inventory FROM player_profiles WHERE user_id=$1 FOR UPDATE`, user).Scan(&raw); err != nil {
		return out, err
	}
	var previous []byte
	err = tx.QueryRowContext(ctx, `SELECT response FROM profession_craft_operations WHERE user_id=$1 AND command_id=$2`, user, command).Scan(&previous)
	if err == nil {
		if err = json.Unmarshal(previous, &out); err != nil {
			return out, err
		}
		if out.CharacterID != character || out.RecipeCode != code || out.Quantity != quantity {
			return CraftResult{}, ErrCraftReplay
		}
		if out.ProgressionVersion < ProfessionProgressionVersion {
			out.Level, out.XP, err = legacyProfessionProgress(out.Level, out.XP)
			if err != nil {
				return CraftResult{}, err
			}
			out.ProgressionVersion = ProfessionProgressionVersion
		}
		return out, nil
	}
	if err != sql.ErrNoRows {
		return out, err
	}
	var owned bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM player_characters WHERE id=$1 AND user_id=$2)`, character, user).Scan(&owned); err != nil {
		return out, err
	}
	if !owned {
		return out, ErrProfessionCharacter
	}
	var guild bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM player_base_buildings WHERE user_id=$1 AND guild_level>0)`, user).Scan(&guild); err != nil {
		return out, err
	}
	if !guild {
		return out, ErrProfessionGuild
	}
	var prof string
	var level int
	var xp int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(processing_code,''),processing_level,processing_xp FROM character_professions WHERE character_id=$1 FOR UPDATE`, character).Scan(&prof, &level, &xp); err != nil {
		return out, err
	}
	if prof != r.Profession {
		return out, ErrCraftProfession
	}
	if level < r.MinLevel {
		return out, ErrCraftLevel
	}
	inv, err := parseStorage(raw)
	if err != nil {
		return out, err
	}
	for _, in := range r.Ingredients {
		id, _, _, _, e := craftResourceTx(ctx, tx, in.Code)
		if e != nil {
			return out, e
		}
		key := fmt.Sprintf("resource_%d", id)
		if marketStackEntryRestricted(inv[key]) {
			return out, ErrCraftIngredients
		}
		left := storageCount(inv[key]) - in.Quantity*quantity
		if left < 0 {
			return out, ErrCraftIngredients
		}
		if left == 0 {
			delete(inv, key)
		} else {
			inv[key]["item_count"] = left
		}
	}
	createdItems := []string{}
	if r.Category == "equipment" || r.Category == "jewelry" {
		for i := 0; i < quantity; i++ {
			var itemID string
			e := tx.QueryRowContext(ctx, `INSERT INTO item_instances(template_id,owner_user_id,source,durability,max_durability) SELECT it.id,$2,'craft',150,150 FROM item_templates it JOIN item_sets s ON s.id=it.set_id WHERE it.code=$1 AND it.rarity=$6 AND s.rarity=$6 AND s.code=$3 AND it.class_restriction=$4 AND s.class_restriction=$4 AND it.slot=$5 RETURNING id::text`, r.Code, user, craftEquipmentSetCode(r.Code), r.HeroClass, r.Slot, r.Rarity).Scan(&itemID)
			if e != nil {
				return out, e
			}
			createdItems = append(createdItems, itemID)
			if e = insertItemInstanceEventTx(tx, itemID, "created", nil, &user, nil, nil, r.Slot); e != nil {
				return out, e
			}
		}
	} else {
		id, image, desc, effect, e := craftResourceTx(ctx, tx, r.Code)
		if e != nil {
			return out, e
		}
		key := fmt.Sprintf("resource_%d", id)
		if marketStackEntryRestricted(inv[key]) {
			return out, ErrCraftInvalid
		}
		count := storageCount(inv[key]) + quantity*r.Output
		if count > 1_000_000_000 {
			return out, ErrCraftInvalid
		}
		entry, e := craftOutputEntry(r, id, image, desc, effect, count)
		if e != nil {
			return out, e
		}
		inv[key] = entry
	}
	level, xp, err = AdvanceProfessionXP(level, xp, int64(r.XP*quantity))
	if err != nil {
		return out, err
	}
	updated, err := json.Marshal(inv)
	if err != nil {
		return out, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE player_profiles SET inventory=$2,updated_at=now() WHERE user_id=$1`, user, string(updated)); err != nil {
		return out, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE character_professions SET processing_level=$2,processing_xp=$3,updated_at=now() WHERE character_id=$1`, character, level, xp); err != nil {
		return out, err
	}
	out = CraftResult{ProgressionVersion: ProfessionProgressionVersion, ItemInstances: createdItems, CharacterID: character, RecipeCode: code, Quantity: quantity, ResultQuantity: quantity * r.Output, Level: level, XP: xp, Inventory: inv}
	response, err := json.Marshal(out)
	if err != nil {
		return out, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO profession_craft_operations(user_id,command_id,character_id,recipe_code,quantity,response) VALUES($1,$2,$3,$4,$5,$6)`, user, command, character, code, quantity, string(response)); err != nil {
		return out, err
	}
	return out, tx.Commit()
}
