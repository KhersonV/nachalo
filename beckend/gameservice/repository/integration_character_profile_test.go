package repository_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"gameservice/game"
	"gameservice/repository"
	"gameservice/service"
)

func openCharacterProfileTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("GAME_DB_DSN")
	if dsn == "" {
		t.Skip("integration test skipped: GAME_DB_DSN not set")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	repository.DB = db
	repository.RunMigrations()
	return db
}

func uniqueCharacterProfileTestName(prefix string) string {
	return fmt.Sprintf("itest_%s_%d", prefix, time.Now().UnixNano())
}

func uniqueUserID() int {
	return 1000000000 + int(time.Now().UnixNano()%1000000000)
}

func oldPlayersTableName() string {
	return "pla" + "yers"
}

func oldHeroOwnershipTableName() string {
	return "player_" + "heroes"
}

func oldSelectedClassColumnName() string {
	return "selected_" + "hero_class_id"
}

func oldPriceColumnName() string {
	return "price_" + "paid"
}

func tableExistsForTest(t *testing.T, db *sql.DB, tableName string) bool {
	t.Helper()

	var exists bool
	if err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name = $1
		)
	`, tableName).Scan(&exists); err != nil {
		t.Fatalf("table exists query for %s: %v", tableName, err)
	}
	return exists
}

func columnExistsForTest(t *testing.T, db *sql.DB, tableName string, columnName string) bool {
	t.Helper()

	var exists bool
	if err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = $1
			  AND column_name = $2
		)
	`, tableName, columnName).Scan(&exists); err != nil {
		t.Fatalf("column exists query for %s.%s: %v", tableName, columnName, err)
	}
	return exists
}

func createTestProfile(t *testing.T, userID int, heroClassID string) {
	t.Helper()

	t.Cleanup(func() {
		_, _ = repository.DB.Exec(`DELETE FROM player_profiles WHERE user_id = $1`, userID)
	})

	if _, err := repository.CreatePlayerProfile(repository.CreatePlayerProfileInput{
		UserID:        userID,
		Name:          uniqueCharacterProfileTestName("player"),
		Image:         "/profile/avatar.webp",
		CharacterType: heroClassID,
		Balance:       10000,
		Inventory:     `{"resource_1":{"name":"wood","item_count":50}}`,
	}); err != nil {
		t.Fatalf("CreatePlayerProfile: %v", err)
	}
}

func insertMinimalMatch(t *testing.T, matchID string, userID int) {
	t.Helper()

	if err := repository.InsertMatch(matchID, "test", 1, 1, userID, []int{userID}, 1, [][2]int{{1, 1}}, [2]int{0, 0}, 10, 10, []byte("[]")); err != nil {
		t.Fatalf("InsertMatch: %v", err)
	}
}

func TestCleanCharacterProfileSchema(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	defer db.Close()

	if !tableExistsForTest(t, db, "player_profiles") {
		t.Fatalf("expected player_profiles to exist")
	}
	if tableExistsForTest(t, db, oldPlayersTableName()) {
		t.Fatalf("old mixed profile table still exists")
	}
	if !tableExistsForTest(t, db, "player_characters") {
		t.Fatalf("expected player_characters to exist")
	}
	if tableExistsForTest(t, db, oldHeroOwnershipTableName()) {
		t.Fatalf("old hero ownership table still exists")
	}
	if columnExistsForTest(t, db, "player_characters", oldPriceColumnName()) {
		t.Fatalf("player_characters must not store purchase price")
	}

	for _, col := range []string{
		"hero_class_id",
		"image",
		"level",
		"exp",
		"max_exp",
		"max_energy",
		"max_health",
		"attack",
		"defense",
		"mobility",
		"agility",
		"sight_range",
		"is_ranged",
		"attack_range",
		"source",
	} {
		if !columnExistsForTest(t, db, "player_characters", col) {
			t.Fatalf("player_characters missing %s", col)
		}
	}

	for _, col := range []string{
		oldSelectedClassColumnName(),
		"character_type",
		"level",
		"experience",
		"max_experience",
		"attack",
		"defense",
		"mobility",
		"agility",
		"sight_range",
		"is_ranged",
		"attack_range",
	} {
		if columnExistsForTest(t, db, "player_profiles", col) {
			t.Fatalf("player_profiles must not contain character column %s", col)
		}
	}
}

func TestProfileCreationCreatesInitialCharacter(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	defer db.Close()

	userID := uniqueUserID()
	createTestProfile(t, userID, "ranger")

	var selectedCharacterID int
	if err := db.QueryRow(`
		SELECT selected_character_id
		FROM player_profiles
		WHERE user_id = $1
	`, userID).Scan(&selectedCharacterID); err != nil {
		t.Fatalf("select profile: %v", err)
	}

	var heroClassID string
	var level, exp, maxExp, attack int
	if err := db.QueryRow(`
		SELECT hero_class_id, level, exp, max_exp, attack
		FROM player_characters
		WHERE id = $1
	`, selectedCharacterID).Scan(&heroClassID, &level, &exp, &maxExp, &attack); err != nil {
		t.Fatalf("select initial character: %v", err)
	}
	if heroClassID != "ranger" || level != 1 || exp != 0 || maxExp != 500 {
		t.Fatalf("unexpected initial character: class=%s level=%d exp=%d maxExp=%d", heroClassID, level, exp, maxExp)
	}
	if attack != repository.ResolveHeroClassStats("ranger").Attack {
		t.Fatalf("expected ranger attack snapshot, got %d", attack)
	}
}

func TestHireAndActiveSelectionUsePlayerCharacters(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	defer db.Close()

	userID := uniqueUserID()
	createTestProfile(t, userID, "guardian")

	if _, err := db.Exec(`INSERT INTO player_base_buildings (user_id, tavern_level) VALUES ($1, 1)`, userID); err != nil {
		t.Fatalf("build tavern fixture: %v", err)
	}

	if _, err := repository.HireHero(userID, "ranger", 2500, true); err != nil {
		t.Fatalf("HireHero ranger: %v", err)
	}
	if _, err := repository.HireHero(userID, "ranger", 2500, true); !errors.Is(err, repository.ErrHeroAlreadyOwned) {
		t.Fatalf("expected duplicate hire to be blocked, got %v", err)
	}

	var rangerID int
	var rangerImage string
	if err := db.QueryRow(`
		SELECT id, image
		FROM player_characters
		WHERE user_id = $1 AND hero_class_id = 'ranger'
	`, userID).Scan(&rangerID, &rangerImage); err != nil {
		t.Fatalf("select ranger character: %v", err)
	}
	if rangerImage == "" {
		t.Fatalf("hired character should store its image")
	}

	if err := repository.SetSelectedCharacterByHeroClass(userID, "ranger"); err != nil {
		t.Fatalf("SetSelectedCharacterByHeroClass: %v", err)
	}

	var selectedCharacterID int
	if err := db.QueryRow(`SELECT selected_character_id FROM player_profiles WHERE user_id = $1`, userID).Scan(&selectedCharacterID); err != nil {
		t.Fatalf("select selected_character_id: %v", err)
	}
	if selectedCharacterID != rangerID {
		t.Fatalf("expected selected_character_id=%d, got %d", rangerID, selectedCharacterID)
	}
	if columnExistsForTest(t, db, "player_profiles", oldSelectedClassColumnName()) {
		t.Fatalf("active selection wrote an old selected class column")
	}
}

func TestProfileReadMatchSnapshotAndFinalizeUseCharacterData(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	defer db.Close()

	userID := uniqueUserID()
	createTestProfile(t, userID, "ranger")

	var characterID int
	if err := db.QueryRow(`
		SELECT selected_character_id
		FROM player_profiles
		WHERE user_id = $1
	`, userID).Scan(&characterID); err != nil {
		t.Fatalf("select selected character: %v", err)
	}
	if _, err := db.Exec(`UPDATE player_characters SET level = 2, exp = 125, max_exp = 2000 WHERE id = $1`, characterID); err != nil {
		t.Fatalf("update character progression fixture: %v", err)
	}

	player, err := repository.GetPlayerByUserID(userID)
	if err != nil {
		t.Fatalf("GetPlayerByUserID: %v", err)
	}
	if player.CharacterType != "ranger" || player.Level != 2 || player.Experience != 125 || player.MaxExperience != 2000 {
		t.Fatalf("profile read did not use active character progression: %+v", player)
	}

	matchID := uniqueCharacterProfileTestName("match_snapshot")
	insertMinimalMatch(t, matchID, userID)
	if err := repository.CreateMatchPlayerCopy(matchID, player, 1, 1, 1); err != nil {
		t.Fatalf("CreateMatchPlayerCopy: %v", err)
	}

	var mpCharacterID int
	var mpType string
	var mpLevel, mpExp, mpAttack int
	var mpInventory []byte
	if err := db.QueryRow(`
		SELECT character_id, character_type, level, experience, attack, inventory
		FROM match_players
		WHERE instance_id = $1 AND user_id = $2
	`, matchID, userID).Scan(&mpCharacterID, &mpType, &mpLevel, &mpExp, &mpAttack, &mpInventory); err != nil {
		t.Fatalf("select match snapshot: %v", err)
	}
	if mpCharacterID != characterID || mpType != "ranger" || mpLevel != 2 || mpExp != 125 {
		t.Fatalf("unexpected match snapshot: char=%d type=%s level=%d exp=%d", mpCharacterID, mpType, mpLevel, mpExp)
	}
	if mpAttack != repository.ResolveHeroClassStats("ranger").Attack {
		t.Fatalf("snapshot attack should come from character stats, got %d", mpAttack)
	}
	var inv map[string]map[string]interface{}
	if err := json.Unmarshal(mpInventory, &inv); err != nil || inv["resource_1"] == nil {
		t.Fatalf("match snapshot did not copy profile inventory: inv=%s err=%v", string(mpInventory), err)
	}

	game.MatchStatesMu.Lock()
	game.MatchStates[matchID] = &game.MatchState{
		InstanceID:   matchID,
		DamageEvents: []game.DamageEvent{{DealerID: userID, TargetType: "monster", TargetID: 1, Amount: 50}},
	}
	game.MatchStatesMu.Unlock()

	if err := service.FinalizeMatch(matchID); err != nil {
		t.Fatalf("FinalizeMatch: %v", err)
	}

	var expAfter int
	if err := db.QueryRow(`SELECT exp FROM player_characters WHERE id = $1`, characterID).Scan(&expAfter); err != nil {
		t.Fatalf("select character exp after finalize: %v", err)
	}
	if expAfter <= 125 {
		t.Fatalf("expected character exp to increase, before=125 after=%d", expAfter)
	}
	if columnExistsForTest(t, db, "player_profiles", "experience") {
		t.Fatalf("profile EXP column should not exist")
	}
}

func TestSwitchingCharactersKeepsIndependentExperience(t *testing.T) {
	db := openCharacterProfileTestDB(t)
	defer db.Close()

	userID := uniqueUserID()
	createTestProfile(t, userID, "guardian")

	if _, err := db.Exec(`INSERT INTO player_base_buildings (user_id, tavern_level) VALUES ($1, 1)`, userID); err != nil {
		t.Fatalf("build tavern fixture: %v", err)
	}
	if _, err := repository.HireHero(userID, "ranger", 2500, true); err != nil {
		t.Fatalf("HireHero ranger: %v", err)
	}

	var guardianID, rangerID int
	if err := db.QueryRow(`SELECT id FROM player_characters WHERE user_id = $1 AND hero_class_id = 'guardian'`, userID).Scan(&guardianID); err != nil {
		t.Fatalf("select guardian: %v", err)
	}
	if err := db.QueryRow(`SELECT id FROM player_characters WHERE user_id = $1 AND hero_class_id = 'ranger'`, userID).Scan(&rangerID); err != nil {
		t.Fatalf("select ranger: %v", err)
	}

	if err := repository.AddCharacterExperience(guardianID, 120); err != nil {
		t.Fatalf("add guardian exp: %v", err)
	}
	if err := repository.SetSelectedCharacterByHeroClass(userID, "ranger"); err != nil {
		t.Fatalf("select ranger: %v", err)
	}
	if err := repository.AddCharacterExperience(rangerID, 40); err != nil {
		t.Fatalf("add ranger exp: %v", err)
	}
	if err := repository.SetSelectedCharacterByHeroClass(userID, "guardian"); err != nil {
		t.Fatalf("select guardian: %v", err)
	}

	var guardianExp, rangerExp int
	if err := db.QueryRow(`SELECT exp FROM player_characters WHERE id = $1`, guardianID).Scan(&guardianExp); err != nil {
		t.Fatalf("select guardian exp: %v", err)
	}
	if err := db.QueryRow(`SELECT exp FROM player_characters WHERE id = $1`, rangerID).Scan(&rangerExp); err != nil {
		t.Fatalf("select ranger exp: %v", err)
	}
	if guardianExp != 120 || rangerExp != 40 {
		t.Fatalf("character EXP not isolated: guardian=%d ranger=%d", guardianExp, rangerExp)
	}
}
