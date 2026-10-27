package balance

import "testing"

// Prepared for the owner; not executed during implementation.
func TestV34IncomingEventOrder(t *testing.T) {
	for _, row := range []struct {
		name string
		hit V34Incoming
		draw float64
		damage, shield, draws int
	}{
		{"hatred_main", V34Incoming{Kind:"basic",Damage:100,Attacker:"guardian",Defender:"berserker",Started:true}, .9,116,0,1},
		{"counter_no_hatred", V34Incoming{Kind:"counter",Damage:100,Attacker:"guardian",Defender:"berserker",Started:true}, .9,100,0,0},
		{"dodge_before_glance", V34Incoming{Kind:"basic",Damage:100,Attacker:"berserker",Defender:"ranger",Started:true}, .08,0,0,1},
		{"glance", V34Incoming{Kind:"basic",Damage:100,Attacker:"guardian",Defender:"guardian",Started:true,Distance:1}, .1,55,0,1},
		{"surge_exclusive", V34Incoming{Kind:"basic",Damage:100,Attacker:"guardian",Defender:"guardian",Started:true,Distance:1}, .2,145,0,1},
		{"opening_on_counter", V34Incoming{Kind:"counter",Damage:100,Defender:"mystic"}, .9,82,0,0},
		{"tick_exempts_opening_then_scales_then_buffer", V34Incoming{Kind:"aura_tick",Damage:2,Attack:18,Defender:"mystic",Shield:1}, .9,2,0,0},
		{"combined_overload_round_once", V34Incoming{Kind:"overload",Damage:8,Attack:30,Defender:"mystic",Shield:2,Started:true}, .9,8,0,0},
		{"opening_before_extra_scale", V34Incoming{Kind:"overload",Damage:8,Attack:30,Defender:"mystic",Shield:2}, .9,7,0,0},
		{"close_ward_before_buffer", V34Incoming{Kind:"basic",Damage:100,Attack:60,Attacker:"berserker",Defender:"mystic",Distance:1,Started:true,Shield:2}, .9,97,0,1},
		{"focus_original_defense", V34Incoming{Kind:"basic",Damage:100,Attacker:"ranger",Defender:"guardian",Distance:2,Defense:100,Started:true}, .9,101,0,1},
		{"zero_preserves_buffer", V34Incoming{Kind:"basic",Damage:0,Defender:"mystic",Shield:2}, .9,0,2,0},
	} {
		t.Run(row.name,func(t *testing.T) {
			cfg:=ExperimentalV34Config()
			draws:=0
			got:=ResolveV34Damage(&cfg,row.hit,func() float64 {draws++;return row.draw})
			if got.Damage!=row.damage || got.Shield!=row.shield || draws!=row.draws {t.Fatalf("got %+v, draws %d; want %d/%d/%d",got,draws,row.damage,row.shield,row.draws)}
		})
	}
}

func TestV34PersistentRandomStream(t *testing.T) {
	previous:=-1.0
	for counter:=int64(0);counter<16;counter++ {
		first:=V34RandomUnit(34,counter)
		if first<0 || first>=1 || first!=V34RandomUnit(34,counter) || first==previous {t.Fatalf("invalid deterministic draw at %d",counter)}
		previous=first
	}
}
