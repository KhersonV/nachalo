package balance

import "math"

func ResearchRound(value float64) int { return int(math.RoundToEven(value)) }

type V34Incoming struct {
	Kind string // basic, counter, followup, collision, overload, aura_tick, aura_tick_extra, aura_exit.
	Damage int
	Attacker, Defender string
	Distance, Attack, Defense int
	Started bool
	Shield int
}

type V34Effect struct { Kind string; Amount int }
type V34Damage struct { Damage, Shield int; Effects []V34Effect }

// Source order: Hatred/dodge -> focus/resists -> one glance/surge draw -> opening
// -> close ward -> fixed-extra ATK scaling -> arcane buffer. No recursive procs.
func ResolveV34Damage(cfg *V34ResearchConfig, hit V34Incoming, draw func() float64) V34Damage {
	out := V34Damage{Damage:hit.Damage,Shield:hit.Shield}
	if out.Damage <= 0 { return out }
	add := func(kind string, amount int) { out.Effects=append(out.Effects,V34Effect{kind,amount}) }
	if hit.Kind == "basic" {
		if damage,applied:=V34HatredDamage(out.Damage,hit.Attacker,hit.Defender,cfg); applied {
			add("hatred",damage-out.Damage); out.Damage=damage
		}
		// Python roll uses randrange(100), even for fractional chance: 8.27 means 9 outcomes.
		if hit.Defender=="ranger" && float64(int(draw()*100)) < cfg.Mechanics["dodge"]*100 {
			add("dodge",out.Damage); out.Damage=0
		}
		if out.Damage>0 {
			if hit.Attacker=="ranger" && hit.Distance>=2 {
				focus:=ResearchRound(float64(hit.Defense-8)*cfg.Mechanics["ranger_armor_focus"])
				if focus>0 { out.Damage+=focus; add("armorFocus",focus) }
			}
			resist,kind:=0.0,""
			if hit.Defender=="guardian" && hit.Distance==1 { resist,kind=cfg.Mechanics["guardian_melee_resist"],"meleeWard" }
			if hit.Defender=="mystic" && hit.Distance>=2 { resist,kind=cfg.Mechanics["mystic_ranged_resist"],"rangedWard" }
			if kind!="" { next:=ResearchRound(float64(out.Damage)*(1-resist)); if next!=out.Damage { add(kind,out.Damage-next) }; out.Damage=next }
		}
		if out.Damage>0 {
			random:=draw()
			if random<cfg.Mechanics["glance"] { next:=ResearchRound(float64(out.Damage)*.55); add("glance",out.Damage-next);out.Damage=next
			} else if random<cfg.Mechanics["glance"]+cfg.Mechanics["surge"] { next:=ResearchRound(float64(out.Damage)*1.45);add("surge",next-out.Damage);out.Damage=next }
		}
	}
	if !hit.Started && cfg.Mechanics["opening"]>0 && hit.Kind!="aura_tick" && hit.Kind!="aura_tick_extra" {
		next:=ResearchRound(float64(out.Damage)*(1-cfg.Mechanics["opening"]))
		if next!=out.Damage { add("openingWard",out.Damage-next) };out.Damage=next
	}
	if hit.Kind=="basic" && out.Damage>0 && hit.Defender=="mystic" && hit.Distance==1 {
		ward:=ResearchRound(float64(hit.Attack-17)*cfg.Mechanics["close_ward"])
		if ward>0 { next:=out.Damage-ward;if next<1 {next=1};add("closeWard",out.Damage-next);out.Damage=next }
	}
	if hit.Kind=="aura_tick" || hit.Kind=="aura_tick_extra" || hit.Kind=="aura_exit" {
		out.Damage=ResearchRound(float64(out.Damage)*float64(hit.Attack)/float64(cfg.BaseAttack[0]))
	} else if hit.Kind=="overload" {
		out.Damage=ResearchRound(float64(out.Damage)*float64(hit.Attack)/float64(cfg.BaseAttack[3]))
	}
	if hit.Defender=="mystic" && out.Damage>0 && out.Shield>0 {
		absorbed:=out.Damage;if absorbed>out.Shield {absorbed=out.Shield}
		out.Shield-=absorbed;out.Damage-=absorbed;add("arcaneWard",absorbed)
	}
	if out.Damage<0 {out.Damage=0}
	return out
}

// SplitMix64: one persisted counter per match, consumed inside the match lock.
// This reproduces Go action streams, not Python's Mersenne Twister sequence.
func V34RandomUnit(seed int64, counter int64) float64 {
	z:=uint64(seed)+uint64(counter+1)*0x9e3779b97f4a7c15
	z=(z^(z>>30))*0xbf58476d1ce4e5b9
	z=(z^(z>>27))*0x94d049bb133111eb
	z^=z>>31
	return float64(z>>11)/9007199254740992.0
}
