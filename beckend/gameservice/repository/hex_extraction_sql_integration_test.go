package repository

import (
	"context"
	"encoding/json"
	"testing"
)

// PostgreSQL regression for the exact Stage 6 participant-result statement.
// Set STAGE2_TEST_DSN to an isolated database to run it locally.
func TestHexQuestParticipantResultUpsertPostgres(t *testing.T) {
	db, _ := hexTestDatabase(t)
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE hex_player_results(instance_id text NOT NULL,user_id integer NOT NULL,result jsonb NOT NULL,PRIMARY KEY(instance_id,user_id)) ON COMMIT DROP`); err != nil {
		t.Fatal(err)
	}
	const instanceID = "quest-upsert-regression"
	insert := func(userID int, result string) {
		t.Helper()
		if _, err := tx.ExecContext(ctx, hexQuestParticipantResultUpsertSQL, instanceID, userID, []byte(result), "quest_artifact_extracted_by_opponent", 701, 2); err != nil {
			t.Fatal(err)
		}
	}
	// The insert path must prepare the otherwise untyped $4 parameter.
	insert(702, `{"method":"opponent_extracted","goldEarned":0}`)
	// Existing paid/defeat outcomes must retain their payouts and loot.
	if _, err := tx.ExecContext(ctx, `INSERT INTO hex_player_results(instance_id,user_id,result) VALUES($1,$2,$3::jsonb)`, instanceID, 703, `{"method":"paid","goldEarned":7,"transferred":{"resource_1":{"item_count":2}}}`); err != nil {
		t.Fatal(err)
	}
	insert(703, `{"method":"opponent_extracted","goldEarned":0}`)
	for _, tc := range []struct {
		userID int
		method string
		gold   int
	}{{702, "opponent_extracted", 0}, {703, "paid", 7}} {
		var raw []byte
		if err := tx.QueryRowContext(ctx, `SELECT result FROM hex_player_results WHERE instance_id=$1 AND user_id=$2`, instanceID, tc.userID).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var result struct {
			Method       string         `json:"method"`
			Reason       string         `json:"reason"`
			WinnerUserID int            `json:"winnerUserId"`
			WinnerTeamID int            `json:"winnerTeamId"`
			GoldEarned   int            `json:"goldEarned"`
			Transferred  map[string]any `json:"transferred"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		if result.Method != tc.method || result.GoldEarned != tc.gold || result.Reason != "quest_artifact_extracted_by_opponent" || result.WinnerUserID != 701 || result.WinnerTeamID != 2 {
			t.Fatalf("user %d result %+v", tc.userID, result)
		}
		if tc.userID == 703 && len(result.Transferred) != 1 {
			t.Fatalf("prior paid loot lost: %+v", result)
		}
	}
}
