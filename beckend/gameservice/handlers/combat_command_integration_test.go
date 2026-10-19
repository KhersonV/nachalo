package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"auth/common"
	"gameservice/game"
	"gameservice/repository"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

func TestAtomicCombatBranchesPostgres(t *testing.T) {
	dsn := os.Getenv("STAGE2_TEST_DSN")
	if dsn == "" {
		t.Skip("set STAGE2_TEST_DSN to isolated PostgreSQL")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("combat_tx_%d", time.Now().UnixNano())
	if _, err = admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, e := url.Parse(dsn)
		if e != nil {
			t.Fatal(e)
		}
		q := u.Query()
		q.Set("options", "-csearch_path="+schema)
		u.RawQuery = q.Encode()
		dsn = u.String()
	} else {
		dsn += " options='-csearch_path=" + schema + "'"
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	old := repository.DB
	repository.DB = db
	defer func() { repository.DB = old }()
	setup := []string{
		`CREATE TABLE matches(instance_id text primary key,active_user_id int,map_width int,map_height int,state_revision bigint default 0,map_revision bigint default 0,quest_artifact_id int default 0,turn_number int default 1,turn_order jsonb default '[1,2]'::jsonb)`,
		`CREATE TABLE match_players(instance_id text,user_id int,character_type text,position jsonb,energy int,max_energy int,health int,max_health int,attack int,defense int,agility int,is_ranged bool,attack_range int,group_id int,name text default '',image text default '',character_id int,inventory jsonb default '{}'::jsonb,level int default 1,experience int default 0,max_experience int default 100,mobility int default 3,sight_range int default 3,balance int default 0,primary key(instance_id,user_id))`,
		`CREATE TABLE artifacts(id int primary key,name text)`,
		`CREATE TABLE inventory_items(instance_id text,user_id int,item_type text,item_id int,item_count int)`,
		`CREATE TABLE match_map_cells(instance_id text,cell_id serial,x int,y int,tile_code int,resource jsonb,barbel jsonb,monster jsonb,is_portal bool default false,is_player bool default false,structure_type text default '',structure_owner_user_id int default 0,structure_health int default 0,structure_defense int default 0,structure_attack int default 0,structure_energy int default 0,is_under_construction bool default false,construction_turns_left int default 0,primary key(instance_id,cell_id),unique(instance_id,x,y))`,
		`CREATE TABLE match_monsters(instance_id text,monster_instance_id int,monster_ref_id int,pos_x int,pos_y int,health int,max_health int,attack int,defense int,speed int,maneuverability int,vision int,image text,primary key(instance_id,monster_instance_id))`,
		`CREATE TABLE player_profiles(user_id int primary key)`,
		`CREATE TABLE item_sets(id int primary key,code text,name text,rarity text,class_restriction text)`,
		`CREATE TABLE item_templates(id bigint primary key,set_id int,code text,name text,rarity text,image_url text,slot text,item_type text,class_restriction text)`,
		`CREATE TABLE item_instances(id uuid default gen_random_uuid() primary key,template_id bigint,owner_user_id int,source text,match_instance_id text)`,
		`CREATE TABLE item_instance_events(item_instance_id uuid,event_type text,from_user_id int,to_user_id int,from_character_id int,to_character_id int,metadata jsonb)`,
	}
	for _, q := range setup {
		if _, err = db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []string{
		`INSERT INTO player_profiles(user_id) VALUES(1)`,
		`INSERT INTO item_sets(id,code,name,rarity,class_restriction) VALUES(1,'aegiswarden_set','Aegis','green','guardian')`,
		`INSERT INTO item_templates(id,set_id,code,name,rarity,image_url,slot,item_type,class_restriction) VALUES(1,1,'aegis_helm','Helm','green','helm.png','head','armor','guardian')`,
	} {
		if _, err = db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	migration, err := os.ReadFile("../../../docs/hex-migration/stage2_commands.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(migration)); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"success", "rollback", "race", "double", "guardianShield", "pvpMystic", "pvpBerserker", "pvpRanger", "pvpDeath", "pvpHTTPDeath", "pvpAutoDeath", "pvpDeathRollback", "pvpDeathRace", "pvpMysticDeath", "pvpBerserkerDeath", "pvpCounterDeath", "pvpCounterRace"} {
		if _, err = db.Exec(`INSERT INTO matches(instance_id,active_user_id,map_width,map_height) VALUES($1,1,2,2)`, id); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`INSERT INTO match_players(instance_id,user_id,character_type,position,energy,max_energy,health,max_health,attack,defense,agility,is_ranged,attack_range,group_id) VALUES($1,1,'guardian','{"x":0,"y":0}',20,20,30,30,8,3,0,false,1,0),($1,2,'ranger','{"x":1,"y":0}',10,10,30,30,5,2,0,true,3,0)`, id); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`INSERT INTO match_map_cells(instance_id,x,y,tile_code,is_player) VALUES($1,0,0,48,true),($1,1,0,48,true),($1,0,1,48,false),($1,1,1,48,false)`, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"pvpCounterDeath", "pvpCounterRace"} {
		if _, err = db.Exec(`UPDATE match_players SET health=2 WHERE instance_id=$1 AND user_id=1`, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"pvpDeath", "pvpHTTPDeath", "pvpAutoDeath", "pvpDeathRollback", "pvpDeathRace", "pvpMysticDeath", "pvpBerserkerDeath"} {
		if _, err = db.Exec(`UPDATE match_players SET health=4 WHERE instance_id=$1 AND user_id=2`, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`UPDATE match_players SET character_type='mystic',is_ranged=true,attack_range=3 WHERE instance_id='pvpMysticDeath' AND user_id=1`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE match_players SET character_type='berserker',health=20 WHERE instance_id='pvpBerserkerDeath' AND user_id=1`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE match_players SET energy=0 WHERE instance_id='pvpBerserkerDeath' AND user_id=2`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE match_players SET character_type='guardian' WHERE instance_id='guardianShield' AND user_id=2`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE match_players SET character_type='mystic',is_ranged=true,attack_range=3 WHERE instance_id='pvpMystic' AND user_id=1`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE match_players SET energy=1 WHERE instance_id='pvpMystic' AND user_id=2`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE match_players SET character_type='berserker',health=20 WHERE instance_id='pvpBerserker' AND user_id=1`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE match_players SET character_type='ranger',is_ranged=true,attack_range=3 WHERE instance_id='pvpRanger' AND user_id=1`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE match_players SET position='{"x":2,"y":0}' WHERE instance_id='pvpRanger' AND user_id=2`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE matches SET map_width=3 WHERE instance_id='pvpRanger'`); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"pve", "pveCollision", "pveCollisionKill", "pveRollback", "pveMystic", "pveBerserker", "pveRanger", "auto", "pveDeath", "dropRollback", "doubleDeath", "autoDeath", "pveCounterDeath", "pveCounterRollback", "internalMonsterDeath", "internalMonsterCollision", "internalMonsterRollback", "internalMonsterCounter", "internalMonsterCounterKill"} {
		if _, err = db.Exec(`INSERT INTO matches(instance_id,active_user_id,map_width,map_height) VALUES($1,1,2,1)`, id); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`INSERT INTO match_players(instance_id,user_id,character_type,position,energy,max_energy,health,max_health,attack,defense,agility,is_ranged,attack_range,group_id) VALUES($1,1,'guardian','{"x":0,"y":0}',20,20,30,30,8,3,0,false,1,0)`, id); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`INSERT INTO match_monsters VALUES($1,7,1,1,0,30,30,5,2,1,1,1,'monster.png')`, id); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`INSERT INTO match_map_cells(instance_id,x,y,tile_code,monster,is_player) VALUES($1,0,0,48,NULL,true),($1,1,0,48,'{"db_instance_id":7,"health":30}',false)`, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"pveCounterDeath", "pveCounterRollback", "internalMonsterDeath", "internalMonsterCollision", "internalMonsterRollback", "internalMonsterCounter", "internalMonsterCounterKill"} {
		if _, err = db.Exec(`UPDATE matches SET map_height=2 WHERE instance_id=$1`, id); err != nil {
			t.Fatal(err)
		}
		if id != "internalMonsterCounter" && id != "internalMonsterCounterKill" {
			if _, err = db.Exec(`UPDATE match_players SET health=2 WHERE instance_id=$1 AND user_id=1`, id); err != nil {
				t.Fatal(err)
			}
		}
		if _, err = db.Exec(`INSERT INTO match_players(instance_id,user_id,character_type,position,energy,max_energy,health,max_health,attack,defense,agility,is_ranged,attack_range,group_id) VALUES($1,2,'ranger','{"x":0,"y":1}',5,10,30,30,5,2,0,true,3,0)`, id); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`INSERT INTO match_map_cells(instance_id,x,y,tile_code,is_player) VALUES($1,0,1,48,true)`, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"pveCollision", "pveCollisionKill", "internalMonsterCollision"} {
		if _, err = db.Exec(`UPDATE match_monsters SET monster_instance_id=1 WHERE instance_id=$1`, id); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`UPDATE match_map_cells SET monster=jsonb_set(monster,'{db_instance_id}','1'::jsonb) WHERE instance_id=$1 AND monster IS NOT NULL`, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`UPDATE match_monsters SET health=4 WHERE instance_id='internalMonsterCounterKill'`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE match_map_cells SET monster='{"db_instance_id":7,"health":4}' WHERE instance_id='internalMonsterCounterKill' AND monster IS NOT NULL`); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"pveDeath", "pveCollisionKill", "dropRollback", "doubleDeath", "autoDeath"} {
		if _, err = db.Exec(`UPDATE match_monsters SET health=4 WHERE instance_id=$1`, id); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`UPDATE match_map_cells SET monster='{"db_instance_id":7,"health":4}' WHERE instance_id=$1 AND monster IS NOT NULL`, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`UPDATE match_map_cells SET monster='{"db_instance_id":1,"health":4}' WHERE instance_id='pveCollisionKill' AND monster IS NOT NULL`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE match_players SET character_type='mystic',is_ranged=true,attack_range=3 WHERE instance_id='pveMystic'`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE matches SET map_width=3 WHERE instance_id='pveMystic'`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE match_monsters SET pos_x=2 WHERE instance_id='pveMystic'`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE match_map_cells SET x=2 WHERE instance_id='pveMystic' AND monster IS NOT NULL`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE match_players SET character_type='berserker',health=20 WHERE instance_id='pveBerserker'`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE match_players SET character_type='ranger',is_ranged=true,attack_range=3 WHERE instance_id='pveRanger'`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE matches SET map_width=3 WHERE instance_id='pveRanger'`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE match_monsters SET pos_x=2 WHERE instance_id='pveRanger'`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE match_map_cells SET x=2 WHERE instance_id='pveRanger' AND monster IS NOT NULL`); err != nil {
		t.Fatal(err)
	}
	oldBroadcast := broadcastFn
	var mu sync.Mutex
	var messages [][]byte
	broadcastFn = func(b []byte) {
		mu.Lock()
		defer mu.Unlock()
		var event struct {
			Payload struct {
				InstanceID    string `json:"instanceId"`
				StateRevision int64  `json:"stateRevision"`
			} `json:"payload"`
		}
		_ = json.Unmarshal(b, &event)
		if event.Payload.StateRevision > 0 {
			var persisted int64
			if queryErr := db.QueryRow(`SELECT state_revision FROM matches WHERE instance_id=$1`, event.Payload.InstanceID).Scan(&persisted); queryErr != nil || persisted < event.Payload.StateRevision {
				t.Errorf("event before commit: persisted=%d err=%v", persisted, queryErr)
			}
		}
		messages = append(messages, append([]byte(nil), b...))
	}
	defer func() { broadcastFn = oldBroadcast }()
	oldRoll := rollReflexProc
	defer func() { rollReflexProc = oldRoll }()
	call := func(instance, command string) (int, atomicAttackResponse, bool) {
		req := AttackRequest{InstanceID: instance, AttackerType: "player", AttackerID: 1, TargetType: "player", TargetID: 2, CommandID: command}
		w := httptest.NewRecorder()
		handled := tryAtomicNonlethalPvP(w, httptest.NewRequest("POST", "/attack", nil), req)
		var response atomicAttackResponse
		_ = json.Unmarshal(w.Body.Bytes(), &response)
		return w.Code, response, handled
	}
	code, result, handled := call("success", "first")
	if !handled || code != 200 || result.DamageToTarget != 6 || result.CounterDamage != 2 || result.StateRevision != 1 || result.MapRevision != 0 {
		t.Fatalf("attack: status=%d result=%+v handled=%v", code, result, handled)
	}
	var successHP, successEnergy int
	if err = db.QueryRow(`SELECT health,energy FROM match_players WHERE instance_id='success' AND user_id=1`).Scan(&successHP, &successEnergy); err != nil || successHP != 28 || successEnergy != 14 {
		t.Fatalf("attacker balance hp=%d energy=%d err=%v", successHP, successEnergy, err)
	}
	var persistedCombat game.ResultEvents
	var persistedCombatRaw []byte
	if err = db.QueryRow(`SELECT combat_result_events FROM matches WHERE instance_id='success'`).Scan(&persistedCombatRaw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(persistedCombatRaw, &persistedCombat); err != nil {
		t.Fatal(err)
	}
	if len(persistedCombat.DamageEvents) != 2 || persistedCombat.DamageEvents[0].Amount != 6 || persistedCombat.DamageEvents[1].Amount != 2 || persistedCombat.DamageEvents[0].SourceType != game.ResultSourcePlayer || persistedCombat.DamageEvents[1].SourceType != game.ResultSourcePlayer {
		t.Fatalf("persisted pvp events=%+v", persistedCombat)
	}
	if err = db.QueryRow(`SELECT health,energy FROM match_players WHERE instance_id='success' AND user_id=2`).Scan(&successHP, &successEnergy); err != nil || successHP != 24 || successEnergy != 8 {
		t.Fatalf("defender balance hp=%d energy=%d err=%v", successHP, successEnergy, err)
	}
	code, replay, handled := call("success", "first")
	if !handled || code != 200 || !reflect.DeepEqual(replay, result) {
		t.Fatalf("replay: status=%d result=%+v", code, replay)
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM match_commands WHERE instance_id='success'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("ledger count=%d err=%v", count, err)
	}
	mu.Lock()
	eventCount := len(messages)
	mu.Unlock()
	if eventCount == 0 {
		t.Fatal("no post-commit event")
	}
	conflictReq := AttackRequest{InstanceID: "success", AttackerType: "player", AttackerID: 1, TargetType: "player", TargetID: 99, CommandID: "first"}
	conflictW := httptest.NewRecorder()
	if !tryAtomicNonlethalPvP(conflictW, httptest.NewRequest("POST", "/attack", nil), conflictReq) || conflictW.Code != 409 {
		t.Fatalf("payload conflict status=%d", conflictW.Code)
	}
	if _, err = db.Exec(`CREATE FUNCTION fail_combat_ledger() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.instance_id='rollback' THEN RAISE EXCEPTION 'injected failure'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER fail_combat BEFORE INSERT ON match_commands FOR EACH ROW EXECUTE FUNCTION fail_combat_ledger()`); err != nil {
		t.Fatal(err)
	}
	code, _, handled = call("rollback", "failed")
	if !handled || code != 500 {
		t.Fatalf("rollback status=%d handled=%v", code, handled)
	}
	var health, energy int
	if err = db.QueryRow(`SELECT health,energy FROM match_players WHERE instance_id='rollback' AND user_id=1`).Scan(&health, &energy); err != nil || health != 30 || energy != 20 {
		t.Fatalf("attacker rollback health=%d energy=%d err=%v", health, energy, err)
	}
	if err = db.QueryRow(`SELECT health,energy FROM match_players WHERE instance_id='rollback' AND user_id=2`).Scan(&health, &energy); err != nil || health != 30 || energy != 10 {
		t.Fatalf("target rollback health=%d energy=%d err=%v", health, energy, err)
	}
	mu.Lock()
	afterFailure := len(messages)
	mu.Unlock()
	if afterFailure != eventCount {
		t.Fatal("rollback published events")
	}
	pveCall := func(instance, command string) (int, atomicAttackResponse, bool) {
		req := AttackRequest{InstanceID: instance, AttackerType: "player", AttackerID: 1, TargetType: "monster", TargetID: 7, CommandID: command}
		w := httptest.NewRecorder()
		handled := tryAtomicNonlethalPvE(w, httptest.NewRequest("POST", "/attack", nil), req)
		var response atomicAttackResponse
		_ = json.Unmarshal(w.Body.Bytes(), &response)
		return w.Code, response, handled
	}
	collisionCall := func(instance, command string) (int, atomicAttackResponse, bool) {
		req := AttackRequest{InstanceID: instance, AttackerType: "player", AttackerID: 1, TargetType: "monster", TargetID: 1, CommandID: command}
		w := httptest.NewRecorder()
		handled := tryAtomicNonlethalPvE(w, httptest.NewRequest("POST", "/attack", nil), req)
		var response atomicAttackResponse
		_ = json.Unmarshal(w.Body.Bytes(), &response)
		return w.Code, response, handled
	}
	code, pve, handled := pveCall("pve", "pve-hit")
	if !handled || code != 200 || pve.DamageToTarget != 6 || pve.NewTargetHP != 22 || pve.CounterDamage != 2 || pve.StateRevision != 1 || pve.MapRevision != 1 {
		t.Fatalf("pve status=%d result=%+v handled=%v", code, pve, handled)
	}
	code, collision, handled := collisionCall("pveCollision", "same-id-hit")
	if !handled || code != 200 || collision.DamageToTarget != 6 || collision.CounterDamage != 2 {
		t.Fatalf("same-id pve status=%d result=%+v handled=%v", code, collision, handled)
	}
	if err = db.QueryRow(`SELECT combat_result_events FROM matches WHERE instance_id='pveCollision'`).Scan(&persistedCombatRaw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(persistedCombatRaw, &persistedCombat); err != nil {
		t.Fatal(err)
	}
	if len(persistedCombat.DamageEvents) != 3 || persistedCombat.DamageEvents[0].SourceType != game.ResultSourcePlayer || persistedCombat.DamageEvents[1].SourceType != game.ResultSourceMonster || persistedCombat.DamageEvents[2].SourceType != game.ResultSourcePlayer || persistedCombat.DamageEvents[0].DealerID != 1 || persistedCombat.DamageEvents[1].DealerID != 1 {
		t.Fatalf("same-id source attribution=%+v", persistedCombat.DamageEvents)
	}
	collisionXP, collisionRewards, _, _, collisionTotal, _, _, collisionTaken, calcErr := game.CalculateResultsFromEvents(persistedCombat, 1, true)
	if calcErr != nil || collisionXP != 1 || collisionTotal != 8 || collisionTaken != 2 || len(collisionRewards) != 1 || collisionRewards[0].Amount != 20 {
		t.Fatalf("same-id rewards xp=%d total=%d taken=%d rewards=%+v err=%v", collisionXP, collisionTotal, collisionTaken, collisionRewards, calcErr)
	}
	code, collisionReplay, handled := collisionCall("pveCollision", "same-id-hit")
	if !handled || code != 200 || !reflect.DeepEqual(collisionReplay, collision) {
		t.Fatalf("same-id replay status=%d result=%+v", code, collisionReplay)
	}
	var monsterHP, cellHP int
	if err = db.QueryRow(`SELECT health FROM match_monsters WHERE instance_id='pve' AND monster_instance_id=7`).Scan(&monsterHP); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT (monster->>'health')::int FROM match_map_cells WHERE instance_id='pve' AND x=1`).Scan(&cellHP); err != nil {
		t.Fatal(err)
	}
	if monsterHP != 22 || cellHP != 22 {
		t.Fatalf("pve monster=%d cell=%d", monsterHP, cellHP)
	}
	code, pveReplay, handled := pveCall("pve", "pve-hit")
	if !handled || code != 200 || !reflect.DeepEqual(pveReplay, pve) {
		t.Fatalf("pve replay status=%d result=%+v", code, pveReplay)
	}
	code, mystic, handled := pveCall("pveMystic", "mystic-hit")
	if !handled || code != 200 || mystic.DamageToTarget != 7 || mystic.NewTargetHP != 23 || mystic.CounterDamage != 0 || mystic.MapRevision != 1 {
		t.Fatalf("mystic pve status=%d result=%+v handled=%v", code, mystic, handled)
	}
	rollReflexProc = func(int) bool { return true }
	code, berserkerPvE, handled := pveCall("pveBerserker", "pve-feast")
	rollReflexProc = oldRoll
	if !handled || code != 200 || berserkerPvE.DamageToTarget != 6 || berserkerPvE.NewTargetHP != 21 || berserkerPvE.CounterDamage != 1 || berserkerPvE.NewAttackerHP != 22 {
		t.Fatalf("berserker pve status=%d result=%+v", code, berserkerPvE)
	}
	game.CreateMatchState("pveRanger", []int{1})
	defer game.DeleteMatchStateInMemory("pveRanger")
	rollReflexProc = func(int) bool { return true }
	code, rangerPvE, handled := pveCall("pveRanger", "pve-crit")
	rollReflexProc = oldRoll
	if !handled || code != 200 || rangerPvE.DamageToTarget != 8 || rangerPvE.NewTargetHP != 22 || rangerPvE.CounterDamage != 0 {
		t.Fatalf("ranger pve status=%d result=%+v", code, rangerPvE)
	}
	if ms, ok := game.GetMatchState("pveRanger"); !ok || ms.GetArmorBreakState("monster", 7).Stacks != 1 {
		t.Fatal("monster armor break missing")
	}
	if _, err = db.Exec(`CREATE FUNCTION fail_pve_map() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.instance_id='pveRollback' THEN RAISE EXCEPTION 'injected map failure'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER fail_pve_map BEFORE UPDATE ON match_map_cells FOR EACH ROW EXECUTE FUNCTION fail_pve_map()`); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	beforePveFailure := len(messages)
	mu.Unlock()
	code, _, handled = pveCall("pveRollback", "pve-fail")
	if !handled || code != 500 {
		t.Fatalf("pve rollback status=%d handled=%v", code, handled)
	}
	if err = db.QueryRow(`SELECT health,energy FROM match_players WHERE instance_id='pveRollback' AND user_id=1`).Scan(&health, &energy); err != nil || health != 30 || energy != 20 {
		t.Fatalf("pve rollback player hp=%d energy=%d err=%v", health, energy, err)
	}
	if err = db.QueryRow(`SELECT health FROM match_monsters WHERE instance_id='pveRollback' AND monster_instance_id=7`).Scan(&monsterHP); err != nil || monsterHP != 30 {
		t.Fatalf("pve rollback monster hp=%d err=%v", monsterHP, err)
	}
	mu.Lock()
	afterPveFailure := len(messages)
	mu.Unlock()
	if afterPveFailure != beforePveFailure {
		t.Fatal("pve rollback published events")
	}
	t.Setenv("EQUIPMENT_DROP_CHANCE_OVERRIDE", "100")
	code, lethal, handled := pveCall("pveDeath", "kill")
	if !handled || code != 200 || lethal.NewTargetHP != 0 || lethal.StateRevision != 1 || lethal.MapRevision != 1 || len(lethal.Drops) != 1 {
		t.Fatalf("lethal pve: code=%d handled=%v response=%+v", code, handled, lethal)
	}
	if err = db.QueryRow(`SELECT count(*) FROM match_monsters WHERE instance_id='pveDeath'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("dead monster row count=%d err=%v", count, err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM match_map_cells WHERE instance_id='pveDeath' AND x=1 AND (monster IS NULL OR monster='null'::jsonb) AND tile_code=48`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("dead monster cell count=%d err=%v", count, err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM item_instances WHERE match_instance_id='pveDeath'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("death drop count=%d err=%v", count, err)
	}
	if err = db.QueryRow(`SELECT combat_result_events FROM matches WHERE instance_id='pveDeath'`).Scan(&persistedCombatRaw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(persistedCombatRaw, &persistedCombat); err != nil {
		t.Fatal(err)
	}
	if len(persistedCombat.KillEvents) != 1 || persistedCombat.KillEvents[0].VictimID != 7 {
		t.Fatalf("persisted kill events=%+v", persistedCombat)
	}
	code, collisionKill, handled := collisionCall("pveCollisionKill", "same-id-kill")
	if !handled || code != 200 || collisionKill.NewTargetHP != 0 {
		t.Fatalf("same-id monster kill status=%d result=%+v handled=%v", code, collisionKill, handled)
	}
	if err = db.QueryRow(`SELECT combat_result_events FROM matches WHERE instance_id='pveCollisionKill'`).Scan(&persistedCombatRaw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(persistedCombatRaw, &persistedCombat); err != nil {
		t.Fatal(err)
	}
	var collisionMonsterKills int
	_, collisionRewards, _, collisionMonsterKills, _, _, _, _, calcErr = game.CalculateResultsFromEvents(persistedCombat, 1, true)
	if calcErr != nil || len(persistedCombat.KillEvents) != 1 || persistedCombat.KillEvents[0].SourceType != game.ResultSourcePlayer || persistedCombat.KillEvents[0].KillerID != 1 || persistedCombat.KillEvents[0].VictimID != 1 || collisionMonsterKills != 1 || len(collisionRewards) != 1 || collisionRewards[0].Amount != 28 {
		t.Fatalf("same-id monster kill events=%+v rewards=%+v count=%d err=%v", persistedCombat, collisionRewards, collisionMonsterKills, calcErr)
	}
	if err = db.QueryRow(`SELECT health,energy FROM match_players WHERE instance_id='pveDeath' AND user_id=1`).Scan(&health, &energy); err != nil || health != 30 || energy != 14 {
		t.Fatalf("lethal attack vitals=%d/%d err=%v", health, energy, err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM item_instance_events e JOIN item_instances i ON i.id=e.item_instance_id WHERE i.match_instance_id='pveDeath' AND e.event_type='created'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("drop event count=%d err=%v", count, err)
	}
	code, replayDeath, handled := pveCall("pveDeath", "kill")
	if !handled || code != 200 || !reflect.DeepEqual(replayDeath, lethal) {
		t.Fatalf("death replay: code=%d handled=%v response=%+v", code, handled, replayDeath)
	}
	if err = db.QueryRow(`SELECT count(*) FROM item_instances WHERE match_instance_id='pveDeath'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate death drop count=%d err=%v", count, err)
	}
	if err = db.QueryRow(`SELECT energy FROM match_players WHERE instance_id='pveDeath' AND user_id=1`).Scan(&energy); err != nil || energy != 14 {
		t.Fatalf("duplicate death energy=%d err=%v", energy, err)
	}
	var deathWG sync.WaitGroup
	deathWG.Add(2)
	go func() { defer deathWG.Done(); _, _, _ = pveCall("doubleDeath", "first") }()
	go func() { defer deathWG.Done(); _, _, _ = pveCall("doubleDeath", "second") }()
	deathWG.Wait()
	var persistedState, persistedMap int64
	if err = db.QueryRow(`SELECT count(*) FROM item_instances WHERE match_instance_id='doubleDeath'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("concurrent death drops=%d err=%v", count, err)
	}
	if err = db.QueryRow(`SELECT state_revision,map_revision FROM matches WHERE instance_id='doubleDeath'`).Scan(&persistedState, &persistedMap); err != nil || persistedState != 1 || persistedMap != 1 {
		t.Fatalf("concurrent death revisions=%d/%d err=%v", persistedState, persistedMap, err)
	}
	if _, err = db.Exec(`CREATE FUNCTION fail_death_drop() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.match_instance_id='dropRollback' THEN RAISE EXCEPTION 'injected drop failure'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER fail_death_drop BEFORE INSERT ON item_instances FOR EACH ROW EXECUTE FUNCTION fail_death_drop()`); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	beforePveFailure = len(messages)
	mu.Unlock()
	code, _, handled = pveCall("dropRollback", "fail-drop")
	if !handled || code != 500 {
		t.Fatalf("drop rollback code=%d handled=%v", code, handled)
	}
	if err = db.QueryRow(`SELECT health,energy FROM match_players WHERE instance_id='dropRollback' AND user_id=1`).Scan(&health, &energy); err != nil || health != 30 || energy != 20 {
		t.Fatalf("drop rollback player=%d/%d err=%v", health, energy, err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM match_monsters WHERE instance_id='dropRollback' AND health=4`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("drop rollback monster=%d err=%v", count, err)
	}
	if err = db.QueryRow(`SELECT state_revision,map_revision FROM matches WHERE instance_id='dropRollback'`).Scan(&persistedState, &persistedMap); err != nil || persistedState != 0 || persistedMap != 0 {
		t.Fatalf("drop rollback revisions=%d/%d err=%v", persistedState, persistedMap, err)
	}
	mu.Lock()
	afterPveFailure = len(messages)
	mu.Unlock()
	if afterPveFailure != beforePveFailure {
		t.Fatal("drop rollback published events")
	}
	game.CreateMatchState("auto", []int{1})
	defer game.DeleteMatchStateInMemory("auto")
	game.CreateMatchState("autoDeath", []int{1})
	defer game.DeleteMatchStateInMemory("autoDeath")
	autoCall := func(instance, command string) (int, atomicAttackResponse) {
		body, _ := json.Marshal(map[string]interface{}{"new_pos_x": 1, "new_pos_y": 0, "commandId": command})
		r := httptest.NewRequest("POST", "/game/auto/player/1/move", bytes.NewReader(body))
		r = mux.SetURLVars(r, map[string]string{"instance_id": instance, "id": "1"})
		r = r.WithContext(context.WithValue(r.Context(), common.UserIDKey, 1))
		w := httptest.NewRecorder()
		MoveOrAttackHandler(w, r)
		var response atomicAttackResponse
		_ = json.Unmarshal(w.Body.Bytes(), &response)
		return w.Code, response
	}
	code, auto := autoCall("auto", "auto-hit")
	if code != 200 || auto.NewTargetHP != 22 || auto.StateRevision != 1 {
		t.Fatalf("autoattack status=%d response=%+v", code, auto)
	}
	code, autoReplay := autoCall("auto", "auto-hit")
	if code != 200 || !reflect.DeepEqual(autoReplay, auto) {
		t.Fatalf("autoattack replay status=%d response=%+v", code, autoReplay)
	}
	code, lethalAuto := autoCall("autoDeath", "auto-kill")
	if code != 200 || lethalAuto.NewTargetHP != 0 || lethalAuto.StateRevision != 1 || lethalAuto.MapRevision != 1 || len(lethalAuto.Drops) != 1 {
		t.Fatalf("lethal autoattack code=%d response=%+v", code, lethalAuto)
	}
	code, autoReplay = autoCall("autoDeath", "auto-kill")
	if code != 200 || !reflect.DeepEqual(autoReplay, lethalAuto) {
		t.Fatalf("lethal autoattack replay code=%d response=%+v", code, autoReplay)
	}
	var attacks sync.WaitGroup
	attacks.Add(2)
	go func() { defer attacks.Done(); _, _, _ = call("double", "a") }()
	go func() { defer attacks.Done(); _, _, _ = call("double", "b") }()
	attacks.Wait()
	var targetHP int
	if err = db.QueryRow(`SELECT health FROM match_players WHERE instance_id='double' AND user_id=2`).Scan(&targetHP); err != nil || targetHP != 18 {
		t.Fatalf("two attacks target HP=%d err=%v", targetHP, err)
	}
	// A move and an attack use the same match row. The winning order is
	// observable as consecutive revisions, or the attack is rejected by range.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _, _ = call("race", "attack") }()
	go func() {
		defer wg.Done()
		_, _ = repository.ApplySquareMove(context.Background(), "race", 1, "move", 0, 1, 3)
	}()
	wg.Wait()
	var state int64
	if err = db.QueryRow(`SELECT state_revision FROM matches WHERE instance_id='race'`).Scan(&state); err != nil || state < 1 || state > 2 {
		t.Fatalf("race revision=%d err=%v", state, err)
	}
	rollReflexProc = oldRoll
	rollReflexProc = func(int) bool { return true }
	code, blocked, handled := call("guardianShield", "blocked")
	rollReflexProc = oldRoll
	if !handled || code != 200 || blocked.DamageToTarget != 0 || blocked.NewTargetHP != 30 || blocked.CounterDamage != 2 {
		t.Fatalf("guardian shield status=%d result=%+v", code, blocked)
	}
	if err = db.QueryRow(`SELECT energy FROM match_players WHERE instance_id='guardianShield' AND user_id=2`).Scan(&energy); err != nil || energy != 10 {
		t.Fatalf("guardian counter energy=%d err=%v", energy, err)
	}
	rollReflexProc = func(int) bool { return true }
	code, mysticPvP, handled := call("pvpMystic", "drain")
	rollReflexProc = oldRoll
	if !handled || code != 200 || mysticPvP.DamageToTarget != 6 || mysticPvP.NewTargetHP != 21 || mysticPvP.CounterDamage != 0 {
		t.Fatalf("mystic pvp status=%d result=%+v", code, mysticPvP)
	}
	if err = db.QueryRow(`SELECT energy FROM match_players WHERE instance_id='pvpMystic' AND user_id=1`).Scan(&energy); err != nil || energy != 17 {
		t.Fatalf("mystic energy=%d err=%v", energy, err)
	}
	if err = db.QueryRow(`SELECT energy FROM match_players WHERE instance_id='pvpMystic' AND user_id=2`).Scan(&energy); err != nil || energy != 0 {
		t.Fatalf("drained energy=%d err=%v", energy, err)
	}
	rollReflexProc = func(int) bool { return true }
	code, berserker, handled := call("pvpBerserker", "blood-feast")
	rollReflexProc = oldRoll
	if !handled || code != 200 || berserker.DamageToTarget != 6 || berserker.NewTargetHP != 21 || berserker.CounterDamage != 2 || berserker.NewAttackerHP != 21 {
		t.Fatalf("berserker pvp status=%d result=%+v", code, berserker)
	}
	game.CreateMatchState("pvpRanger", []int{1, 2})
	defer game.DeleteMatchStateInMemory("pvpRanger")
	rollReflexProc = func(int) bool { return true }
	code, ranger, handled := call("pvpRanger", "crit")
	rollReflexProc = oldRoll
	if !handled || code != 200 || ranger.DamageToTarget != 8 || ranger.NewTargetHP != 22 || ranger.CounterDamage != 0 {
		t.Fatalf("ranger pvp status=%d result=%+v", code, ranger)
	}
	if ms, ok := game.GetMatchState("pvpRanger"); !ok || ms.GetArmorBreakState("player", 2).Stacks != 1 {
		t.Fatal("ranger armor break missing")
	}
	matchmaking := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer matchmaking.Close()
	t.Setenv("MATCHMAKING_URL", matchmaking.URL)
	for _, id := range []string{"pvpDeath", "pvpHTTPDeath", "pvpAutoDeath", "pvpDeathRollback", "pvpDeathRace", "pvpMysticDeath", "pvpBerserkerDeath", "pvpCounterDeath", "pvpCounterRace", "pveCounterDeath", "pveCounterRollback", "internalMonsterDeath", "internalMonsterCollision", "internalMonsterRollback", "internalMonsterCounter", "internalMonsterCounterKill"} {
		game.CreateMatchState(id, []int{1, 2})
		defer game.DeleteMatchStateInMemory(id)
		defer CancelTurnTimer(id)
	}
	httpAttack, _ := json.Marshal(AttackRequest{CommandID: "full-handler-kill", AttackerType: "player", AttackerID: 1, TargetType: "player", TargetID: 2, InstanceID: "pvpHTTPDeath"})
	httpReq := httptest.NewRequest("POST", "/attack", bytes.NewReader(httpAttack))
	httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), common.UserIDKey, 1))
	httpRecorder := httptest.NewRecorder()
	UniversalAttackHandler(httpRecorder, httpReq)
	var httpDeath atomicAttackResponse
	_ = json.Unmarshal(httpRecorder.Body.Bytes(), &httpDeath)
	if httpRecorder.Code != 200 || httpDeath.NewTargetHP != 0 || httpDeath.StateRevision != 1 || httpDeath.MapRevision != 1 {
		t.Fatalf("full attack handler direct death status=%d response=%+v", httpRecorder.Code, httpDeath)
	}
	code, autoPvPDeath := autoCall("pvpAutoDeath", "auto-pvp-kill")
	if code != 200 || autoPvPDeath.NewTargetHP != 0 || autoPvPDeath.StateRevision != 1 || autoPvPDeath.MapRevision != 1 {
		t.Fatalf("pvp autoattack death status=%d response=%+v", code, autoPvPDeath)
	}
	code, autoPvPReplay := autoCall("pvpAutoDeath", "auto-pvp-kill")
	if code != 200 || !reflect.DeepEqual(autoPvPReplay, autoPvPDeath) {
		t.Fatalf("pvp autoattack death replay status=%d response=%+v", code, autoPvPReplay)
	}
	code, death, handled := call("pvpDeath", "fatal-hit")
	if !handled || code != 200 || death.NewTargetHP != 0 || death.CounterDamage != 0 || death.StateRevision != 1 || death.MapRevision != 1 {
		t.Fatalf("direct pvp death: status=%d handled=%t response=%+v", code, handled, death)
	}
	var deathHP, deathEnergy, deathActive int
	var deathOrder []byte
	var deathCell bool
	if err = db.QueryRow(`SELECT p.health,m.active_user_id,m.turn_order,c.is_player FROM matches m JOIN match_players p ON p.instance_id=m.instance_id AND p.user_id=2 JOIN match_map_cells c ON c.instance_id=m.instance_id AND c.x=1 AND c.y=0 WHERE m.instance_id='pvpDeath'`).Scan(&deathHP, &deathActive, &deathOrder, &deathCell); err != nil || deathHP != 0 || deathActive != 1 || string(deathOrder) != `[1]` || deathCell {
		t.Fatalf("death persisted hp=%d active=%d order=%s occupied=%t err=%v", deathHP, deathActive, deathOrder, deathCell, err)
	}
	if err = db.QueryRow(`SELECT energy FROM match_players WHERE instance_id='pvpDeath' AND user_id=1`).Scan(&deathEnergy); err != nil || deathEnergy != 20 {
		t.Fatalf("death regen energy=%d err=%v", deathEnergy, err)
	}
	if err = db.QueryRow(`SELECT combat_result_events FROM matches WHERE instance_id='pvpDeath'`).Scan(&persistedCombatRaw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(persistedCombatRaw, &persistedCombat); err != nil || len(persistedCombat.DamageEvents) != 1 || len(persistedCombat.KillEvents) != 1 || len(persistedCombat.DefeatedUsers) != 1 || persistedCombat.DefeatedUsers[0] != 2 {
		t.Fatalf("death history=%+v err=%v", persistedCombat, err)
	}
	code, replay, handled = call("pvpDeath", "fatal-hit")
	if !handled || code != 200 || !reflect.DeepEqual(replay, death) {
		t.Fatalf("death replay status=%d response=%+v", code, replay)
	}
	if err = db.QueryRow(`SELECT combat_result_events FROM matches WHERE instance_id='pvpDeath'`).Scan(&persistedCombatRaw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(persistedCombatRaw, &persistedCombat); err != nil || len(persistedCombat.KillEvents) != 1 {
		t.Fatalf("death replay duplicated history=%+v err=%v", persistedCombat, err)
	}
	for _, id := range []string{"pvpMysticDeath", "pvpBerserkerDeath"} {
		code, classDeath, classHandled := call(id, "fatal-class")
		if !classHandled || code != 200 || classDeath.NewTargetHP != 0 || classDeath.StateRevision != 1 || classDeath.MapRevision != 1 {
			t.Fatalf("%s direct class death status=%d handled=%t response=%+v", id, code, classHandled, classDeath)
		}
	}
	if _, err = db.Exec(`CREATE FUNCTION fail_pvp_death_ledger() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.instance_id='pvpDeathRollback' THEN RAISE EXCEPTION 'injected death failure'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER fail_pvp_death_ledger BEFORE INSERT ON match_commands FOR EACH ROW EXECUTE FUNCTION fail_pvp_death_ledger()`); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	beforeDeathFailure := len(messages)
	mu.Unlock()
	code, _, handled = call("pvpDeathRollback", "fatal-hit")
	if !handled || code != 500 {
		t.Fatalf("death rollback status=%d handled=%t", code, handled)
	}
	mu.Lock()
	afterDeathFailure := len(messages)
	mu.Unlock()
	if beforeDeathFailure != afterDeathFailure {
		t.Fatal("death rollback published WebSocket events")
	}
	var rollbackState, rollbackMap int64
	if err = db.QueryRow(`SELECT p.health,m.active_user_id,m.turn_order,c.is_player,m.state_revision,m.map_revision,m.combat_result_events FROM matches m JOIN match_players p ON p.instance_id=m.instance_id AND p.user_id=2 JOIN match_map_cells c ON c.instance_id=m.instance_id AND c.x=1 AND c.y=0 WHERE m.instance_id='pvpDeathRollback'`).Scan(&deathHP, &deathActive, &deathOrder, &deathCell, &rollbackState, &rollbackMap, &persistedCombatRaw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(persistedCombatRaw, &persistedCombat); err != nil || deathHP != 4 || deathActive != 1 || string(deathOrder) != `[1, 2]` || !deathCell || rollbackState != 0 || rollbackMap != 0 || len(persistedCombat.KillEvents) != 0 || len(persistedCombat.DefeatedUsers) != 0 {
		t.Fatalf("death rollback hp=%d active=%d order=%s cell=%t revisions=%d/%d events=%+v err=%v", deathHP, deathActive, deathOrder, deathCell, rollbackState, rollbackMap, persistedCombat, err)
	}
	if err = db.QueryRow(`SELECT energy FROM match_players WHERE instance_id='pvpDeathRollback' AND user_id=1`).Scan(&deathEnergy); err != nil || deathEnergy != 20 {
		t.Fatalf("death rollback energy=%d err=%v", deathEnergy, err)
	}
	if ms, ok := game.GetMatchState("pvpDeathRollback"); !ok || len(ms.SnapshotResultEvents().DefeatedUsers) != 0 || len(ms.TurnOrder) != 2 {
		t.Fatal("death rollback changed process-local MatchState")
	}
	var deathRace sync.WaitGroup
	deathRace.Add(2)
	go func() { defer deathRace.Done(); _, _, _ = call("pvpDeathRace", "fatal-a") }()
	go func() { defer deathRace.Done(); _, _, _ = call("pvpDeathRace", "fatal-b") }()
	deathRace.Wait()
	if err = db.QueryRow(`SELECT state_revision,combat_result_events FROM matches WHERE instance_id='pvpDeathRace'`).Scan(&rollbackState, &persistedCombatRaw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(persistedCombatRaw, &persistedCombat); err != nil || rollbackState != 1 || len(persistedCombat.KillEvents) != 1 || len(persistedCombat.DefeatedUsers) != 1 {
		t.Fatalf("competing death commands revision=%d history=%+v err=%v", rollbackState, persistedCombat, err)
	}
	code, counterDeath, handled := call("pvpCounterDeath", "counter-kill")
	if !handled || code != 200 || counterDeath.NewAttackerHP != 0 || counterDeath.CounterDamage != 2 || counterDeath.StateRevision != 1 || counterDeath.MapRevision != 1 {
		t.Fatalf("pvp counter death status=%d handled=%t response=%+v", code, handled, counterDeath)
	}
	if err = db.QueryRow(`SELECT p.health,p.energy,m.active_user_id,m.turn_order,c.is_player,m.combat_result_events FROM matches m JOIN match_players p ON p.instance_id=m.instance_id AND p.user_id=1 JOIN match_map_cells c ON c.instance_id=m.instance_id AND c.x=0 AND c.y=0 WHERE m.instance_id='pvpCounterDeath'`).Scan(&deathHP, &deathEnergy, &deathActive, &deathOrder, &deathCell, &persistedCombatRaw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(persistedCombatRaw, &persistedCombat); err != nil || deathHP != 0 || deathEnergy != 14 || deathActive != 2 || string(deathOrder) != `[2]` || deathCell || len(persistedCombat.DamageEvents) != 2 || len(persistedCombat.KillEvents) != 1 || persistedCombat.KillEvents[0].KillerID != 2 || len(persistedCombat.DefeatedUsers) != 1 || persistedCombat.DefeatedUsers[0] != 1 {
		t.Fatalf("pvp counter persistence hp=%d energy=%d active=%d order=%s cell=%t events=%+v err=%v", deathHP, deathEnergy, deathActive, deathOrder, deathCell, persistedCombat, err)
	}
	code, replay, handled = call("pvpCounterDeath", "counter-kill")
	if !handled || code != 200 || !reflect.DeepEqual(replay, counterDeath) {
		t.Fatalf("pvp counter replay code=%d response=%+v", code, replay)
	}
	var counterWG sync.WaitGroup
	counterWG.Add(2)
	go func() { defer counterWG.Done(); _, _, _ = call("pvpCounterRace", "one") }()
	go func() { defer counterWG.Done(); _, _, _ = call("pvpCounterRace", "two") }()
	counterWG.Wait()
	if err = db.QueryRow(`SELECT state_revision,combat_result_events FROM matches WHERE instance_id='pvpCounterRace'`).Scan(&rollbackState, &persistedCombatRaw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(persistedCombatRaw, &persistedCombat); err != nil || rollbackState != 1 || len(persistedCombat.KillEvents) != 1 {
		t.Fatalf("pvp counter race revision=%d events=%+v err=%v", rollbackState, persistedCombat, err)
	}
	code, monsterCounter, handled := pveCall("pveCounterDeath", "monster-counter-kill")
	if !handled || code != 200 || monsterCounter.NewAttackerHP != 0 || monsterCounter.StateRevision != 1 || monsterCounter.MapRevision != 1 {
		t.Fatalf("monster counter death status=%d handled=%t response=%+v", code, handled, monsterCounter)
	}
	if err = db.QueryRow(`SELECT p.health,p.energy,m.active_user_id,m.turn_order,c.is_player,m.combat_result_events FROM matches m JOIN match_players p ON p.instance_id=m.instance_id AND p.user_id=1 JOIN match_map_cells c ON c.instance_id=m.instance_id AND c.x=0 AND c.y=0 WHERE m.instance_id='pveCounterDeath'`).Scan(&deathHP, &deathEnergy, &deathActive, &deathOrder, &deathCell, &persistedCombatRaw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(persistedCombatRaw, &persistedCombat); err != nil || deathHP != 0 || deathEnergy != 14 || deathActive != 2 || string(deathOrder) != `[2]` || deathCell || len(persistedCombat.DamageEvents) != 2 || len(persistedCombat.KillEvents) != 1 || len(persistedCombat.DefeatedUsers) != 1 {
		t.Fatalf("monster counter persistence hp=%d energy=%d active=%d order=%s cell=%t events=%+v err=%v", deathHP, deathEnergy, deathActive, deathOrder, deathCell, persistedCombat, err)
	}
	if err = db.QueryRow(`SELECT mm.health,(c.monster->>'health')::int FROM match_monsters mm JOIN match_map_cells c ON c.instance_id=mm.instance_id AND c.monster->>'db_instance_id'=mm.monster_instance_id::text WHERE mm.instance_id='pveCounterDeath'`).Scan(&monsterHP, &cellHP); err != nil || monsterHP != 24 || cellHP != 24 {
		t.Fatalf("monster counter original hit hp=%d cell=%d err=%v", monsterHP, cellHP, err)
	}
	code, replay, handled = pveCall("pveCounterDeath", "monster-counter-kill")
	if !handled || code != 200 || !reflect.DeepEqual(replay, monsterCounter) {
		t.Fatalf("monster counter replay code=%d response=%+v", code, replay)
	}
	if _, err = db.Exec(`CREATE FUNCTION fail_counter_queue() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.instance_id IN ('pveCounterRollback','internalMonsterRollback') THEN RAISE EXCEPTION 'injected queue failure'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER fail_counter_queue BEFORE UPDATE OF turn_order ON matches FOR EACH ROW EXECUTE FUNCTION fail_counter_queue()`); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	beforeCounterFailure := len(messages)
	mu.Unlock()
	code, _, handled = pveCall("pveCounterRollback", "fail-queue")
	mu.Lock()
	afterCounterFailure := len(messages)
	mu.Unlock()
	if !handled || code != 500 || beforeCounterFailure != afterCounterFailure {
		t.Fatalf("counter rollback code=%d handled=%t events=%d/%d", code, handled, beforeCounterFailure, afterCounterFailure)
	}
	if err = db.QueryRow(`SELECT p.health,p.energy,m.state_revision,m.combat_result_events FROM matches m JOIN match_players p ON p.instance_id=m.instance_id AND p.user_id=1 WHERE m.instance_id='pveCounterRollback'`).Scan(&deathHP, &deathEnergy, &rollbackState, &persistedCombatRaw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(persistedCombatRaw, &persistedCombat); err != nil || deathHP != 2 || deathEnergy != 20 || rollbackState != 0 || len(persistedCombat.DamageEvents) != 0 || len(persistedCombat.KillEvents) != 0 {
		t.Fatalf("counter rollback hp=%d energy=%d revision=%d events=%+v err=%v", deathHP, deathEnergy, rollbackState, persistedCombat, err)
	}
	monsterReq := AttackRequest{InstanceID: "internalMonsterDeath", AttackerType: "monster", AttackerID: 7, TargetType: "player", TargetID: 1, CommandID: "monster-hit"}
	monsterWriter := httptest.NewRecorder()
	universalAttackLocked(monsterWriter, monsterReq)
	var internalResult atomicAttackResponse
	internalErr := json.Unmarshal(monsterWriter.Body.Bytes(), &internalResult)
	if monsterWriter.Code != 200 || internalErr != nil || internalResult.NewTargetHP != 0 || internalResult.StateRevision != 1 || internalResult.MapRevision != 1 {
		t.Fatalf("internal monster death status=%d result=%+v err=%v", monsterWriter.Code, internalResult, internalErr)
	}
	internalReplay, internalErr := internalMonsterAttackCommand(context.Background(), monsterReq)
	if internalErr != nil || !reflect.DeepEqual(&internalResult, internalReplay) {
		t.Fatalf("internal monster replay result=%+v err=%v", internalReplay, internalErr)
	}
	if err = db.QueryRow(`SELECT p.health,m.active_user_id,m.turn_order,c.is_player,m.combat_result_events FROM matches m JOIN match_players p ON p.instance_id=m.instance_id AND p.user_id=1 JOIN match_map_cells c ON c.instance_id=m.instance_id AND c.x=0 AND c.y=0 WHERE m.instance_id='internalMonsterDeath'`).Scan(&deathHP, &deathActive, &deathOrder, &deathCell, &persistedCombatRaw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(persistedCombatRaw, &persistedCombat); err != nil || deathHP != 0 || deathActive != 2 || string(deathOrder) != `[2]` || deathCell || len(persistedCombat.DamageEvents) != 1 || len(persistedCombat.KillEvents) != 1 || len(persistedCombat.DefeatedUsers) != 1 {
		t.Fatalf("internal monster persistence hp=%d active=%d order=%s cell=%t events=%+v err=%v", deathHP, deathActive, deathOrder, deathCell, persistedCombat, err)
	}
	monsterReq.InstanceID, monsterReq.AttackerID, monsterReq.CommandID = "internalMonsterCollision", 1, "same-id-monster-kill"
	collisionMonsterDeath, internalErr := internalMonsterAttackCommand(context.Background(), monsterReq)
	if internalErr != nil || collisionMonsterDeath.NewTargetHP != 0 {
		t.Fatalf("same-id player death result=%+v err=%v", collisionMonsterDeath, internalErr)
	}
	if err = db.QueryRow(`SELECT combat_result_events FROM matches WHERE instance_id='internalMonsterCollision'`).Scan(&persistedCombatRaw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(persistedCombatRaw, &persistedCombat); err != nil {
		t.Fatal(err)
	}
	monsterXP, monsterRewards, monsterPlayerKills, _, monsterTotal, _, _, monsterTaken, calcErr := game.CalculateResultsFromEvents(persistedCombat, 1, false)
	if calcErr != nil || len(persistedCombat.KillEvents) != 1 || persistedCombat.KillEvents[0].SourceType != game.ResultSourceMonster || persistedCombat.KillEvents[0].KillerID != 1 || monsterXP != 0 || monsterPlayerKills != 0 || monsterTotal != 0 || monsterTaken != 2 || len(monsterRewards) != 0 {
		t.Fatalf("same-id player kill events=%+v xp=%d rewards=%+v kills=%d total=%d taken=%d err=%v", persistedCombat, monsterXP, monsterRewards, monsterPlayerKills, monsterTotal, monsterTaken, calcErr)
	}
	if replay, replayErr := internalMonsterAttackCommand(context.Background(), monsterReq); replayErr != nil || !reflect.DeepEqual(replay, collisionMonsterDeath) {
		t.Fatalf("same-id monster replay=%+v err=%v", replay, replayErr)
	}
	monsterReq.AttackerID = 7
	monsterReq.InstanceID, monsterReq.CommandID = "internalMonsterCounter", "monster-counter"
	internalCounter, internalErr := internalMonsterAttackCommand(context.Background(), monsterReq)
	if internalErr != nil || internalCounter.CounterDamage != 6 || internalCounter.StateRevision != 1 {
		t.Fatalf("internal monster counter result=%+v err=%v", internalCounter, internalErr)
	}
	monsterReq.InstanceID, monsterReq.CommandID = "internalMonsterCounterKill", "monster-counter-kill"
	internalCounter, internalErr = internalMonsterAttackCommand(context.Background(), monsterReq)
	if internalErr != nil || internalCounter.NewAttackerHP != 0 || internalCounter.MapRevision != 1 {
		t.Fatalf("internal monster counter kill result=%+v err=%v", internalCounter, internalErr)
	}
	if err = db.QueryRow(`SELECT count(*) FROM match_monsters WHERE instance_id='internalMonsterCounterKill'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("internal monster counter kill rows=%d err=%v", count, err)
	}
	monsterReq.InstanceID, monsterReq.CommandID = "internalMonsterRollback", "monster-rollback"
	mu.Lock()
	beforeCounterFailure = len(messages)
	mu.Unlock()
	_, internalErr = internalMonsterAttackCommand(context.Background(), monsterReq)
	mu.Lock()
	afterCounterFailure = len(messages)
	mu.Unlock()
	if internalErr == nil || beforeCounterFailure != afterCounterFailure {
		t.Fatalf("internal monster rollback err=%v events=%d/%d", internalErr, beforeCounterFailure, afterCounterFailure)
	}
	if err = db.QueryRow(`SELECT p.health,m.state_revision,m.map_revision,m.combat_result_events FROM matches m JOIN match_players p ON p.instance_id=m.instance_id AND p.user_id=1 WHERE m.instance_id='internalMonsterRollback'`).Scan(&deathHP, &rollbackState, &rollbackMap, &persistedCombatRaw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(persistedCombatRaw, &persistedCombat); err != nil || deathHP != 2 || rollbackState != 0 || rollbackMap != 0 || len(persistedCombat.DamageEvents) != 0 || len(persistedCombat.KillEvents) != 0 {
		t.Fatalf("internal monster rollback hp=%d revisions=%d/%d events=%+v err=%v", deathHP, rollbackState, rollbackMap, persistedCombat, err)
	}
	publicBody, _ := json.Marshal(AttackRequest{InstanceID: "internalMonsterCounter", AttackerType: "monster", AttackerID: 7, TargetType: "player", TargetID: 1, CommandID: "forbidden"})
	publicW := httptest.NewRecorder()
	UniversalAttackHandler(publicW, httptest.NewRequest("POST", "/attack", bytes.NewReader(publicBody)))
	if publicW.Code != http.StatusForbidden {
		t.Fatalf("public monster attack status=%d", publicW.Code)
	}
	fresh, openErr := sql.Open("postgres", dsn)
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer fresh.Close()
	repository.DB = fresh
	var restoredDeathRaw []byte
	var historyComplete bool
	if err = fresh.QueryRow(`SELECT combat_result_events,combat_results_complete FROM matches WHERE instance_id='pvpDeath'`).Scan(&restoredDeathRaw, &historyComplete); err != nil {
		t.Fatal(err)
	}
	var restoredDeath game.ResultEvents
	if err = json.Unmarshal(restoredDeathRaw, &restoredDeath); err != nil {
		t.Fatal(err)
	}
	if historyComplete || len(restoredDeath.KillEvents) != 1 || len(restoredDeath.DefeatedUsers) != 1 {
		t.Fatalf("fresh session death history=%+v complete=%t", restoredDeath, historyComplete)
	}
	for _, id := range []string{"pvpCounterDeath", "pveCounterDeath", "internalMonsterDeath"} {
		var freshHP, freshActive int
		var freshEventsRaw []byte
		if err = fresh.QueryRow(`SELECT p.health,m.active_user_id,m.combat_result_events FROM matches m JOIN match_players p ON p.instance_id=m.instance_id AND p.user_id=1 WHERE m.instance_id=$1`, id).Scan(&freshHP, &freshActive, &freshEventsRaw); err != nil {
			t.Fatal(err)
		}
		var freshEvents game.ResultEvents
		if err = json.Unmarshal(freshEventsRaw, &freshEvents); err != nil || freshHP != 0 || freshActive != 2 || len(freshEvents.KillEvents) != 1 || len(freshEvents.DefeatedUsers) != 1 {
			t.Fatalf("fresh counter/death match=%s hp=%d active=%d events=%+v err=%v", id, freshHP, freshActive, freshEvents, err)
		}
	}
	deathXP, deathRewards, playerKills, _, _, _, _, _, calcErr := game.CalculateResultsFromEvents(restoredDeath, 1, true)
	if calcErr != nil || deathXP != 2 || playerKills != 1 || len(deathRewards) != 1 || deathRewards[0].Amount != 40 {
		t.Fatalf("fresh session death rewards xp=%d rewards=%+v kills=%d err=%v", deathXP, deathRewards, playerKills, calcErr)
	}
	if err = fresh.QueryRow(`SELECT state_revision,map_revision FROM matches WHERE instance_id='pve'`).Scan(&persistedState, &persistedMap); err != nil || persistedState != 1 || persistedMap != 1 {
		t.Fatalf("fresh connection revisions=%d/%d err=%v", persistedState, persistedMap, err)
	}
	replayRaw, found, replayErr := repository.ReplayMatchCommand(context.Background(), "pve", "pve-hit", combatCommandCanonical(AttackRequest{AttackerID: 1, TargetID: 7}, "nonlethal-pve-v1"))
	if replayErr != nil || !found || len(replayRaw) == 0 {
		t.Fatalf("fresh connection ledger found=%v err=%v", found, replayErr)
	}
	if err := repository.RestoreMatchStates(); err != nil {
		t.Fatalf("restore result history: %v", err)
	}
	for _, fixture := range []struct {
		instance string
		userID   int
		survived bool
	}{
		{"success", 1, true},
		{"pveCollision", 1, true},
		{"pveCollisionKill", 1, true},
		{"internalMonsterCollision", 1, false},
	} {
		var raw []byte
		if err := fresh.QueryRow(`SELECT combat_result_events FROM matches WHERE instance_id=$1`, fixture.instance).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var persisted game.ResultEvents
		if err := json.Unmarshal(raw, &persisted); err != nil {
			t.Fatal(err)
		}
		beforeXP, beforeRewards, beforePK, beforeMK, beforeTotal, beforePlayers, beforeMonsters, beforeTaken, beforeErr := game.CalculateResultsFromEvents(persisted, fixture.userID, fixture.survived)
		afterXP, afterRewards, afterPK, afterMK, afterTotal, afterPlayers, afterMonsters, afterTaken, afterErr := game.CalculateResults(fixture.instance, fixture.userID, fixture.survived)
		if beforeErr != nil || afterErr != nil || beforeXP != afterXP || !reflect.DeepEqual(beforeRewards, afterRewards) || beforePK != afterPK || beforeMK != afterMK || beforeTotal != afterTotal || beforePlayers != afterPlayers || beforeMonsters != afterMonsters || beforeTaken != afterTaken {
			t.Fatalf("restarted results %s: before=(%d,%+v,%d,%d,%d,%d,%d,%d,%v) after=(%d,%+v,%d,%d,%d,%d,%d,%d,%v)", fixture.instance, beforeXP, beforeRewards, beforePK, beforeMK, beforeTotal, beforePlayers, beforeMonsters, beforeTaken, beforeErr, afterXP, afterRewards, afterPK, afterMK, afterTotal, afterPlayers, afterMonsters, afterTaken, afterErr)
		}
		state, ok := game.GetMatchState(fixture.instance)
		if !ok {
			t.Fatalf("restarted match state missing for %s", fixture.instance)
		}
		restored := state.SnapshotResultEvents()
		if !slices.Equal(restored.DamageEvents, persisted.DamageEvents) || !slices.Equal(restored.KillEvents, persisted.KillEvents) || !slices.Equal(restored.DefeatedUsers, persisted.DefeatedUsers) {
			t.Fatalf("restarted event types lost for %s", fixture.instance)
		}
	}
	repository.DB = db
}
