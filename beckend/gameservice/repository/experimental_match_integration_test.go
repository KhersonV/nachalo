package repository

import (
	"context"
	"testing"
	"gameservice/internal/game/balance"
)

// Prepared only. Uses existing disposable-schema fixture, never run here.
func TestV34FrozenMatchAndRandomReplayPostgres(t *testing.T) {
	t.Setenv("NACHALO_COMBAT_RULESET","legacy")
	db,_:=hexTestDatabase(t)
	ctx:=context.Background()
	legacy,err:=CreateHexTestMatch(ctx,701,97)
	if err!=nil {t.Fatal(err)}
	t.Setenv("NACHALO_COMBAT_RULESET","experimental_v34")
	t.Setenv("NACHALO_COMBAT_SEED","34")
	experimental,err:=CreateHexTestMatch(ctx,701,98)
	if err!=nil {t.Fatal(err)}
	if experimental.Player.MaxHealth!=218 || legacy.Player.MaxHealth!=130 {t.Fatal("new-match stat selection failed")}
	t.Setenv("NACHALO_COMBAT_RULESET","legacy")
	rules,cfg,err:=ReadMatchCombatRulesTx(ctx,db,experimental.InstanceID)
	if err!=nil || rules!=balance.ExperimentalV34 || cfg==nil {t.Fatalf("frozen config: %s %+v %v",rules,cfg,err)}
	if cfg.HatredPercent!=16 {t.Fatal("frozen Hatred changed")}
	oldRules,oldCfg,err:=ReadMatchCombatRulesTx(ctx,db,legacy.InstanceID)
	if err!=nil || oldRules!=balance.Legacy || oldCfg!=nil {t.Fatal("old match was retuned")}
	command,err:=BeginMatchCommand(ctx,experimental.InstanceID,701,"v34-seed-probe","seed-probe")
	if err!=nil {t.Fatal(err)}
	defer command.Rollback()
	counter:=command.RandomCounter
	if command.RandomSeed!=34 || command.RandomUnit()!=balance.V34RandomUnit(34,counter) {t.Fatal("seed was reset/lost")}
	response:=map[string]any{"probe":true}
	if _,_,err=command.Commit(ctx,&response,false);err!=nil {t.Fatal(err)}
	replay,err:=BeginMatchCommand(ctx,experimental.InstanceID,701,"v34-seed-probe","seed-probe")
	if err!=nil {t.Fatal(err)}
	defer replay.Rollback()
	if replay.Replay==nil {t.Fatal("retry did not return receipt")}
	var persisted int64
	if err=db.QueryRowContext(ctx,`SELECT combat_rng_counter FROM matches WHERE instance_id=$1`,experimental.InstanceID).Scan(&persisted);err!=nil {t.Fatal(err)}
	if persisted!=counter+1 {t.Fatalf("retry consumed RNG: %d",persisted)}
	rolledBack,err:=BeginMatchCommand(ctx,experimental.InstanceID,701,"v34-rollback-probe","rollback-probe")
	if err!=nil {t.Fatal(err)}
	rolledBack.RandomUnit()
	rolledBack.Rollback()
	if err=db.QueryRowContext(ctx,`SELECT combat_rng_counter FROM matches WHERE instance_id=$1`,experimental.InstanceID).Scan(&persisted);err!=nil {t.Fatal(err)}
	if persisted!=counter+1 {t.Fatal("rolled-back draw persisted")}
}
