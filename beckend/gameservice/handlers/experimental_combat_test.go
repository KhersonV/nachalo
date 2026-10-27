package handlers

import (
	"testing"
	"gameservice/internal/game/balance"
)

func TestV34CounterFactorBeforeWoundedEscalation(t *testing.T) {
	cfg:=balance.ExperimentalV34Config()
	a:=stats{Rules:&cfg,Attack:17,CharacterType:"berserker"}
	d:=stats{Health:24,MaxHealth:100,Defense:7}
	// Flat 15 -> counter round(15*.5938)=9 -> wounded floor(9*1.35)=12.
	got:=resolveCounterDamage(a,d)
	if got.Damage!=12 || got.NewHealth!=12 {t.Fatalf("counter order: %+v",got)}
	if classReflexChance(stats{Rules:&cfg,Agility:5,CharacterType:"ranger"},stats{CharacterType:"berserker"})!=10 {t.Fatal("legacy proc suppression leaked")}
}
