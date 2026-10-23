package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const ProfessionMaxLevel = 100

var ErrProfessionCharacter = errors.New("profession character not owned")
var ErrProfessionInvalid = errors.New("invalid processing profession")
var ErrProfessionConfirmation = errors.New("profession reset confirmation required")
var ErrProfessionStale = errors.New("profession selection changed")

type ProfessionDefinition struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

type ProfessionProgress struct {
	ProfessionDefinition
	Level       int   `json:"level"`
	XP          int64 `json:"xp"`
	XPToNext    int64 `json:"xpToNext"`
	XPRemaining int64 `json:"xpRemaining"`
}

type CharacterProfessions struct {
	CharacterID int                    `json:"characterId"`
	HeroClassID string                 `json:"heroClassId"`
	Gathering   ProfessionProgress     `json:"gathering"`
	Processing  *ProfessionProgress    `json:"processing"`
	Available   []ProfessionDefinition `json:"available"`
}

// ProfessionXPToNext is the XP required within the current level. At level
// 100 there is no further progression. This never touches combat progression.
func ProfessionXPToNext(level int) int64 {
	if level < 1 || level >= ProfessionMaxLevel {
		return 0
	}
	return 100 * int64(level) * int64(level)
}

// AdvanceProfessionXP is for a future server-side extraction transaction.
// There is intentionally no client API that can call this to award XP.
func AdvanceProfessionXP(level int, xp, awarded int64) (int, int64, error) {
	if level < 1 || level > ProfessionMaxLevel || xp < 0 || awarded < 0 {
		return 0, 0, ErrProfessionInvalid
	}
	if level == ProfessionMaxLevel {
		return level, 0, nil
	}
	if xp >= ProfessionXPToNext(level) {
		return 0, 0, ErrProfessionInvalid
	}
	for awarded > 0 && level < ProfessionMaxLevel {
		needed := ProfessionXPToNext(level) - xp
		if awarded < needed {
			xp += awarded
			awarded = 0
			break
		}
		awarded -= needed
		level++
		xp = 0
	}
	if level == ProfessionMaxLevel {
		xp = 0
	}
	return level, xp, nil
}

func gatheringProfessionForClass(heroClassID string) (string, error) {
	switch heroClassID {
	case "berserker": return "trophy_hunter", nil
	case "ranger": return "tracker", nil
	case "guardian": return "miner", nil
	case "mystic": return "herbalist", nil
	default: return "", fmt.Errorf("unknown hero class %q", heroClassID)
	}
}

func EnsureProfessionsSchema() error {
	_, err := DB.Exec(`
		CREATE TABLE IF NOT EXISTS profession_categories (
			code TEXT PRIMARY KEY,
			name TEXT NOT NULL
		);
		INSERT INTO profession_categories(code,name) VALUES
		('gathering','Добыча'),('processing','Переработка')
		ON CONFLICT (code) DO UPDATE SET name=EXCLUDED.name;
		CREATE TABLE IF NOT EXISTS profession_catalog (
			code TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			category TEXT NOT NULL REFERENCES profession_categories(code),
			sort_order INTEGER NOT NULL,
			enabled BOOLEAN NOT NULL DEFAULT true,
			UNIQUE (code,category)
		);
		CREATE TABLE IF NOT EXISTS character_professions (
			character_id INTEGER PRIMARY KEY REFERENCES player_characters(id) ON DELETE CASCADE,
			gathering_code TEXT NOT NULL,
			gathering_category TEXT NOT NULL DEFAULT 'gathering' CHECK (gathering_category='gathering'),
			gathering_level INTEGER NOT NULL DEFAULT 1 CHECK (gathering_level BETWEEN 1 AND 100),
			gathering_xp BIGINT NOT NULL DEFAULT 0 CHECK (gathering_xp >= 0),
			processing_code TEXT,
			processing_category TEXT NOT NULL DEFAULT 'processing' CHECK (processing_category='processing'),
			processing_level INTEGER NOT NULL DEFAULT 1 CHECK (processing_level BETWEEN 1 AND 100),
			processing_xp BIGINT NOT NULL DEFAULT 0 CHECK (processing_xp >= 0),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			FOREIGN KEY (gathering_code,gathering_category) REFERENCES profession_catalog(code,category),
			FOREIGN KEY (processing_code,processing_category) REFERENCES profession_catalog(code,category),
			CHECK (processing_code IS NOT NULL OR (processing_level=1 AND processing_xp=0))
		);
		INSERT INTO profession_catalog(code,name,description,category,sort_order) VALUES
		('trophy_hunter','Трофейщик','Извлечение трофеев из монстров и сбор находок на HEX-клетках.','gathering',10),
		('tracker','Следопыт','Поиск материалов у монстров и на HEX-клетках.','gathering',20),
		('miner','Рудокоп','Добыча минералов и других материалов в HEX-подземелье.','gathering',30),
		('herbalist','Травник','Сбор трав и материалов в HEX-подземелье.','gathering',40),
		('boneworker','Косторез','Обработка костей и трофеев. Рецепты появятся позже.','processing',10),
		('leatherworker','Кожевник','Обработка кожи. Рецепты появятся позже.','processing',20),
		('blacksmith','Кузнец','Обработка металлов. Рецепты появятся позже.','processing',30),
		('scribe','Писарь','Работа с письменами. Рецепты появятся позже.','processing',40),
		('alchemist','Алхимик','Приготовление составов. Рецепты появятся позже.','processing',50)
		ON CONFLICT (code) DO UPDATE SET name=EXCLUDED.name,description=EXCLUDED.description,sort_order=EXCLUDED.sort_order;
		INSERT INTO character_professions(character_id,gathering_code)
		SELECT id, CASE hero_class_id
			WHEN 'berserker' THEN 'trophy_hunter'
			WHEN 'ranger' THEN 'tracker'
			WHEN 'guardian' THEN 'miner'
			WHEN 'mystic' THEN 'herbalist'
		END
		FROM player_characters
		WHERE hero_class_id IN ('berserker','ranger','guardian','mystic')
		ON CONFLICT (character_id) DO NOTHING;
	`)
	return err
}

// Staging belongs to a match and its character. Stage 2 can write here on
// extraction actions and consume it only in the successful HEX exit transaction.
func EnsureMatchProfessionXPTable() error {
	_, err := DB.Exec(`CREATE TABLE IF NOT EXISTS match_profession_xp (
		match_instance_id TEXT NOT NULL REFERENCES matches(instance_id) ON DELETE CASCADE,
		character_id INTEGER NOT NULL REFERENCES player_characters(id) ON DELETE CASCADE,
		profession_code TEXT NOT NULL REFERENCES profession_catalog(code),
		xp BIGINT NOT NULL DEFAULT 0 CHECK (xp >= 0),
		PRIMARY KEY (match_instance_id,character_id,profession_code)
	)`)
	return err
}

func ensureCharacterProfessionTx(tx *sql.Tx, characterID int, heroClassID string) error {
	code, err := gatheringProfessionForClass(heroClassID)
	if err != nil { return err }
	_, err = tx.Exec(`INSERT INTO character_professions(character_id,gathering_code) VALUES($1,$2) ON CONFLICT(character_id) DO NOTHING`, characterID, code)
	return err
}

func professionProgress(def ProfessionDefinition, level int, xp int64) ProfessionProgress {
	next := ProfessionXPToNext(level)
	remaining := next - xp
	if remaining < 0 { remaining = 0 }
	return ProfessionProgress{ProfessionDefinition: def, Level: level, XP: xp, XPToNext: next, XPRemaining: remaining}
}

// Character ID 0 means the account's currently selected character.
func GetCharacterProfessions(ctx context.Context, userID, characterID int) (CharacterProfessions, error) {
	var result CharacterProfessions
	var gatheringCode string
	var processingCode sql.NullString
	var gatheringLevel, processingLevel int
	var gatheringXP, processingXP int64
	err := DB.QueryRowContext(ctx, `SELECT pc.id,pc.hero_class_id,cp.gathering_code,cp.gathering_level,cp.gathering_xp,cp.processing_code,cp.processing_level,cp.processing_xp
		FROM player_characters pc JOIN player_profiles pp ON pp.user_id=pc.user_id
		JOIN character_professions cp ON cp.character_id=pc.id
		WHERE pc.user_id=$1 AND pc.id=CASE WHEN $2::integer=0 THEN pp.selected_character_id ELSE $2::integer END`, userID, characterID).
		Scan(&result.CharacterID,&result.HeroClassID,&gatheringCode,&gatheringLevel,&gatheringXP,&processingCode,&processingLevel,&processingXP)
	if errors.Is(err,sql.ErrNoRows) { return result, ErrProfessionCharacter }
	if err != nil { return result,err }
	rows,err := DB.QueryContext(ctx, `SELECT code,name,description,category FROM profession_catalog WHERE enabled=true AND (code=$1 OR category='processing') ORDER BY category,sort_order`, gatheringCode)
	if err != nil { return result,err }
	defer rows.Close()
	result.Available = []ProfessionDefinition{}
	for rows.Next() {
		var def ProfessionDefinition
		if err := rows.Scan(&def.Code,&def.Name,&def.Description,&def.Category); err != nil { return result,err }
		if def.Code == gatheringCode { result.Gathering = professionProgress(def,gatheringLevel,gatheringXP) } else if def.Category == "processing" {
			result.Available = append(result.Available,def)
			if processingCode.Valid && def.Code == processingCode.String { progress := professionProgress(def,processingLevel,processingXP); result.Processing = &progress }
		}
	}
	if err := rows.Err(); err != nil { return result,err }
	if result.Gathering.Code == "" || (processingCode.Valid && result.Processing == nil) { return result,ErrProfessionInvalid }
	return result,nil
}

func ChooseProcessingProfession(ctx context.Context, userID, characterID int, code, expectedCode string, confirmReset bool) error {
	if characterID <= 0 || code == "" { return ErrProfessionInvalid }
	tx,err := DB.BeginTx(ctx,nil)
	if err != nil { return err }
	defer tx.Rollback()
	var heroClassID string
	err = tx.QueryRowContext(ctx,`SELECT hero_class_id FROM player_characters WHERE id=$1 AND user_id=$2 FOR UPDATE`,characterID,userID).Scan(&heroClassID)
	if errors.Is(err,sql.ErrNoRows) { return ErrProfessionCharacter }
	if err != nil { return err }
	var available bool
	err = tx.QueryRowContext(ctx,`SELECT enabled AND category='processing' FROM profession_catalog WHERE code=$1`,code).Scan(&available)
	if errors.Is(err,sql.ErrNoRows) || err==nil && !available { return ErrProfessionInvalid }
	if err != nil { return err }
	if err := ensureCharacterProfessionTx(tx,characterID,heroClassID); err != nil { return err }
	var current sql.NullString
	if err := tx.QueryRowContext(ctx,`SELECT processing_code FROM character_professions WHERE character_id=$1 FOR UPDATE`,characterID).Scan(&current); err != nil { return err }
	if current.Valid && current.String == code { return tx.Commit() }
	if current.String != expectedCode { return ErrProfessionStale }
	if current.Valid && !confirmReset { return ErrProfessionConfirmation }
	_,err = tx.ExecContext(ctx,`UPDATE character_professions SET processing_code=$2,processing_level=1,processing_xp=0,updated_at=now() WHERE character_id=$1`,characterID,code)
	if err != nil { return err }
	return tx.Commit()
}
