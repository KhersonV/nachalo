package handlers

// Opt-in validation driver. Every action executes the production transaction,
// permissions, geometry, class abilities and death handling. No combat model here.
import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"
	"time"

	"gameservice/internal/game/balance"
	"gameservice/internal/hexgrid"
	"gameservice/internal/hexmap"
	"gameservice/repository"
)

type v34ValidationRow struct {
	EquipmentModel string                `json:"equipment_model,omitempty"`
	Jewelry        bool                  `json:"jewelry,omitempty"`
	Scenario       string                `json:"scenario,omitempty"`
	Policy         string                `json:"policy,omitempty"`
	Arena          string                `json:"arena"`
	Tier           int                   `json:"tier"`
	A              string                `json:"a"`
	B              string                `json:"b"`
	First          int                   `json:"first"`
	Seed           int64                 `json:"seed"`
	Winner         int                   `json:"winner"` // -1 = turn limit; 0/1 are seats, including mirrors.
	Turns          int                   `json:"turns"`
	Attacks        [2]int                `json:"attacks"`
	Milliseconds   int64                 `json:"milliseconds"`
	Initial        [2]v34ValidationState `json:"initial"`
	Trace          []v34ValidationAction `json:"trace,omitempty"`
}
type v34ValidationState struct {
	User                                                                    int    `json:"user"`
	Class                                                                   string `json:"class"`
	HP, MaxHP, Attack, Defense, Reflex, AP, MP, Reactions, Overload, Shield int
	BreakOwner, BreakStacks, DrainRound, PersonalTurn                       int
	MaxAP, MaxMP, MaxReactions                                              int
	AuraSlowUsed                                                            bool
	AttackRange, SightRange                                                 int
	IsRanged                                                                bool
	Started, Drain, ReflexUsed                                              bool
	Position                                                                hexgrid.Coord
}
type v34ValidationAction struct {
	Command        string                     `json:"command"`
	Kind           string                     `json:"kind"`
	User           int                        `json:"user"`
	Round          int                        `json:"round"`
	RNGBefore      int64                      `json:"rng_before"`
	RNGAfter       int64                      `json:"rng_after"`
	ActiveBefore   int                        `json:"active_before"`
	ActiveAfter    int                        `json:"active_after"`
	Before         [2]v34ValidationState      `json:"before"`
	Path           []hexgrid.Coord            `json:"path,omitempty"`
	AttackResult   *HexAttackResult           `json:"attack_result,omitempty"`
	MovementResult *repository.HexRouteResult `json:"movement_result,omitempty"`
	State          [2]v34ValidationState      `json:"state"`
}

func TestExperimentalV34TargetedActions(t *testing.T) {
	output := os.Getenv("NACHALO_V34_SCENARIOS")
	if output == "" {
		t.Skip("opt in with NACHALO_V34_SCENARIOS")
	}
	if os.Getenv("STAGE2_TEST_DSN") != "postgres://v34_validation@127.0.0.1:55434/nachalo_v34_validation?sslmode=disable" {
		t.Fatal("dedicated validation cluster required")
	}
	t.Setenv("NACHALO_COMBAT_RULESET", "experimental_v34")
	db, _ := hexCombatDatabase(t)
	ctx := context.Background()
	file, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	classes := []string{"guardian", "berserker", "ranger", "mystic"}
	var profiles [4][2]int
	for k, class := range classes {
		for seat := 0; seat < 2; seat++ {
			user := 1200 + k*2 + seat
			profiles[k][seat] = user
			if _, err = repository.CreatePlayerProfile(repository.CreatePlayerProfileInput{UserID: user, Name: fmt.Sprintf("scenario %d", user), CharacterType: class}); err != nil {
				t.Fatal(err)
			}
		}
	}
	cases := []struct {
		name                             string
		a, b, distance                   int
		arena                            string
		attack, hp, stacks, over, shield int
		move                             bool
	}{
		{"third_shot_collision", 2, 3, 2, "tactical", 0, 0, 2, 0, 0, false},
		{"third_shot_push", 2, 3, 2, "open", 0, 0, 2, 0, 0, false},
		{"mystic_third_buffer", 3, 3, 2, "open", 0, 0, 0, 2, 10, false},
		{"berserker_heal_survive", 1, 3, 1, "open", 0, 50, 0, 0, 0, false},
		{"berserker_heal_lethal_counter", 1, 3, 1, "open", 0, 1, 0, 0, 0, false},
		{"mystic_close_ward", 1, 3, 1, "open", 60, 0, 0, 0, 0, false},
		{"guardian_block_before_crit", 2, 0, 2, "open", 0, 0, 0, 0, 0, false},
		{"enemy_voluntary_aura_exit", 1, 0, 1, "open", 0, 0, 0, 0, 0, true},
		{"owner_voluntary_aura_exit_exempt", 0, 1, 1, "open", 0, 0, 0, 0, 0, true},
		{"ap_to_mp_exchange", 0, 1, 4, "open", 0, 0, 0, 0, 0, false},
	}
	for index, tc := range cases {
		if filter := os.Getenv("NACHALO_V34_SCENARIO"); filter != "" && tc.name != filter {
			continue
		}
		seed := int64(3400 + index)
		if tc.name == "guardian_block_before_crit" {
			for seed = 0; int(balance.V34RandomUnit(seed, 0)*100) >= 7; seed++ {
			}
		}
		if tc.a == 1 && tc.hp > 0 {
			for seed = 0; balance.V34RandomUnit(seed, 0) < .32 || int(balance.V34RandomUnit(seed, 1)*100) >= 10; seed++ {
			}
		}
		t.Setenv("NACHALO_COMBAT_SEED", strconv.FormatInt(seed, 10))
		users := [2]int{profiles[tc.a][0], profiles[tc.b][1]}
		id := "scenario-" + tc.name
		_, err = repository.CreateHexMatch(ctx, repository.HexMatchPlan{InstanceID: id, Mode: "hex-test", Teams: []hexmap.Team{{ID: 1, PlayerIDs: []int{users[0]}}, {ID: 2, PlayerIDs: []int{users[1]}}}, TurnOrder: []int{users[0], users[1]}, Seed: 97})
		if err != nil {
			t.Fatal(err)
		}
		v34Arena(t, db, id, tc.arena, users)
		positions := [2]int{-tc.distance, 0}
		if tc.move && tc.a == 1 {
			positions = [2]int{1, 0}
		}
		if tc.move && tc.a == 0 {
			positions = [2]int{0, 1}
		}
		for seat, user := range users {
			if _, err = db.Exec(`UPDATE match_players SET position_q=$3,position_r=0,combat_started=true,personal_turn_counter=1 WHERE instance_id=$1 AND user_id=$2`, id, user, positions[seat]); err != nil {
				t.Fatal(err)
			}
		}
		if _, err = db.Exec(`UPDATE match_map_cells SET is_player=(r=0 AND (q=$2 OR q=$3)) WHERE instance_id=$1`, id, positions[0], positions[1]); err != nil {
			t.Fatal(err)
		}
		if tc.attack > 0 {
			if _, err = db.Exec(`UPDATE match_players SET attack=$3 WHERE instance_id=$1 AND user_id=$2`, id, users[0], tc.attack); err != nil {
				t.Fatal(err)
			}
		}
		if tc.hp > 0 {
			if _, err = db.Exec(`UPDATE match_players SET health=$3 WHERE instance_id=$1 AND user_id=$2`, id, users[0], tc.hp); err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(`UPDATE match_players SET health=30 WHERE instance_id=$1 AND user_id=$2`, id, users[1]); err != nil {
				t.Fatal(err)
			}
		}
		if tc.stacks > 0 {
			if _, err = db.Exec(`UPDATE match_players SET hex_armor_break_owner=$3,hex_armor_break_stacks=$4 WHERE instance_id=$1 AND user_id=$2`, id, users[1], users[0], tc.stacks); err != nil {
				t.Fatal(err)
			}
		}
		if tc.over > 0 {
			if _, err = db.Exec(`UPDATE match_players SET hex_overload_target=$3,hex_overload_count=$4 WHERE instance_id=$1 AND user_id=$2`, id, users[0], fmt.Sprintf("player:%d", users[1]), tc.over); err != nil {
				t.Fatal(err)
			}
		}
		if tc.shield > 0 {
			if _, err = db.Exec(`UPDATE match_players SET combat_magic_shield=$3 WHERE instance_id=$1 AND user_id=$2`, id, users[1], tc.shield); err != nil {
				t.Fatal(err)
			}
		}
		if tc.name == "ap_to_mp_exchange" {
			if _, err = db.Exec(`UPDATE match_players SET mp=0 WHERE instance_id=$1 AND user_id=$2`, id, users[0]); err != nil {
				t.Fatal(err)
			}
		}
		before := v34State(t, db, id, users)
		action := v34ValidationAction{Command: tc.name, User: users[0], Round: 1, Before: before, ActiveBefore: users[0]}
		if tc.name == "ap_to_mp_exchange" {
			action.Kind = "exchange"
			if _, err = repository.ExchangeHexAP(ctx, id, users[0], tc.name); err != nil {
				t.Fatal(err)
			}
		} else if tc.move {
			action.Kind = "move"
			destination := hexgrid.Coord{Q: 2, R: 0}
			if tc.a == 0 {
				destination.Q = -1
			}
			moved, e := repository.ApplyHexMove(ctx, id, users[0], tc.name, destination)
			if e != nil {
				t.Fatal(e)
			}
			action.Path = moved.Path
			action.MovementResult = &moved
		} else {
			action.Kind = "attack"
			hit, e := ApplyHexPlayerAttack(ctx, id, users[0], users[1], tc.name, nil)
			if e != nil {
				t.Fatal(tc.name, e)
			}
			action.AttackResult = &hit
		}
		action.State = v34State(t, db, id, users)
		if err = db.QueryRow(`SELECT combat_rng_counter,COALESCE(active_user_id,0) FROM matches WHERE instance_id=$1`, id).Scan(&action.RNGAfter, &action.ActiveAfter); err != nil {
			t.Fatal(err)
		}
		if tc.move {
			want := before[0].HP
			if tc.a != 0 {
				want -= 2
			}
			if action.State[0].HP != want || action.State[1].HP != before[1].HP {
				t.Fatal("incorrect voluntary aura exit:", tc.name)
			}
		}
		row := v34ValidationRow{Scenario: tc.name, Arena: tc.arena, A: classes[tc.a], B: classes[tc.b], Seed: seed, Winner: -1, Initial: before, Trace: []v34ValidationAction{action}}
		if err = encoder.Encode(row); err != nil {
			t.Fatal(err)
		}
		t.Log(tc.name, "passed")
	}
}

func v34State(t *testing.T, db *sql.DB, id string, users [2]int) [2]v34ValidationState {
	t.Helper()
	var out [2]v34ValidationState
	for i, user := range users {
		s := &out[i]
		s.User = user
		err := db.QueryRow(`SELECT character_type,health,max_health,attack,defense,agility,ap,mp,reactions,hex_overload_count,combat_magic_shield,combat_started,hex_mp_drain,hex_mystic_reflex_used,position_q,position_r,hex_armor_break_owner,hex_armor_break_stacks,hex_mystic_drain_round,personal_turn_counter,max_ap,max_mp,max_reactions,hex_aura_slow_used,attack_range,sight_range,is_ranged FROM match_players WHERE instance_id=$1 AND user_id=$2`, id, user).Scan(&s.Class, &s.HP, &s.MaxHP, &s.Attack, &s.Defense, &s.Reflex, &s.AP, &s.MP, &s.Reactions, &s.Overload, &s.Shield, &s.Started, &s.Drain, &s.ReflexUsed, &s.Position.Q, &s.Position.R, &s.BreakOwner, &s.BreakStacks, &s.DrainRound, &s.PersonalTurn, &s.MaxAP, &s.MaxMP, &s.MaxReactions, &s.AuraSlowUsed, &s.AttackRange, &s.SightRange, &s.IsRanged)
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func v34Arena(t *testing.T, db *sql.DB, id, arena string, users [2]int) ([]hexmap.Cell, map[hexgrid.EdgeKey]bool) {
	t.Helper()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for _, table := range []string{"match_monsters", "match_hex_edges", "match_map_cells"} {
		if _, err = tx.Exec(`DELETE FROM `+table+` WHERE instance_id=$1`, id); err != nil {
			t.Fatal(err)
		}
	}
	cells := []hexmap.Cell{}
	for q := -5; q <= 5; q++ {
		for r := -5; r <= 5; r++ {
			pos := hexgrid.Coord{Q: q, R: r}
			d, _ := hexgrid.Distance(hexgrid.Coord{}, pos)
			if d > 5 {
				continue
			}
			cell := hexmap.Cell{CellID: len(cells) + 1, Position: pos, Terrain: "floor"}
			cells = append(cells, cell)
			if _, err = tx.Exec(`INSERT INTO match_map_cells(instance_id,cell_id,q,r,tile_code,is_player) VALUES($1,$2,$3,$4,48,$5)`, id, cell.CellID, q, r, r == 0 && (q == -2 || q == 2)); err != nil {
				t.Fatal(err)
			}
		}
	}
	walls := map[hexgrid.EdgeKey]bool{}
	if arena == "tactical" {
		for _, r := range []int{-1, 0, 1} {
			p, n := hexgrid.Coord{Q: 0, R: r}, hexgrid.Coord{Q: 1, R: r}
			key, _ := hexgrid.CanonicalEdge(p, n)
			walls[key] = true
			// CanonicalEdge follows the server's first three directions.
			for direction := 0; direction < 6; direction++ {
				next, _ := hexgrid.Neighbor(p, direction)
				if next == n {
					if direction >= 3 {
						p = n
						direction -= 3
					}
					if _, err = tx.Exec(`INSERT INTO match_hex_edges(instance_id,q,r,direction,wall_kind) VALUES($1,$2,$3,$4,'wall')`, id, p.Q, p.R, direction); err != nil {
						t.Fatal(err)
					}
					break
				}
			}
		}
	}
	starts := []hexmap.Start{{UserID: users[0], TeamID: 1, Position: hexgrid.Coord{Q: -2, R: 0}}, {UserID: users[1], TeamID: 2, Position: hexgrid.Coord{Q: 2, R: 0}}}
	raw, _ := json.Marshal(struct {
		Seed   int64          `json:"seed"`
		Cells  []hexmap.Cell  `json:"cells"`
		Starts []hexmap.Start `json:"starts"`
	}{34, cells, starts})
	if _, err = tx.Exec(`UPDATE matches SET map=$2,map_width=11,map_height=11 WHERE instance_id=$1`, id, raw); err != nil {
		t.Fatal(err)
	}
	for i, user := range users {
		if _, err = tx.Exec(`UPDATE match_players SET position_q=$3,position_r=0,personal_turn_counter=0,combat_started=false,combat_magic_shield=CASE WHEN character_type='mystic' THEN round(max_health*.0088) ELSE 0 END WHERE instance_id=$1 AND user_id=$2`, id, user, starts[i].Position.Q); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return cells, walls
}

// Geometry-only bot heuristic: global wall distances avoid local minima. The
// selected route and each movement step still use the existing server BFS and
// real visibility/MP checks. This bot knows the opposing fixture position.
func v34BotDistances(cells []hexmap.Cell, walls map[hexgrid.EdgeKey]bool, target hexgrid.Coord) map[hexgrid.Coord]int {
	floor := map[hexgrid.Coord]bool{}
	for _, cell := range cells {
		floor[cell.Position] = true
	}
	distance := map[hexgrid.Coord]int{target: 0}
	queue := []hexgrid.Coord{target}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for direction := 0; direction < 6; direction++ {
			n, _ := hexgrid.Neighbor(p, direction)
			edge, _ := hexgrid.CanonicalEdge(p, n)
			if _, seen := distance[n]; seen || !floor[n] || walls[edge] {
				continue
			}
			distance[n] = distance[p] + 1
			queue = append(queue, n)
		}
	}
	return distance
}

func TestExperimentalV34Validation(t *testing.T) {
	output := os.Getenv("NACHALO_V34_RESULTS")
	if output == "" {
		t.Skip("opt in with NACHALO_V34_RESULTS and isolated STAGE2_TEST_DSN")
	}
	// Refuse ordinary/application database connections, even if a caller sets DSN incorrectly.
	if os.Getenv("STAGE2_TEST_DSN") != "postgres://v34_validation@127.0.0.1:55434/nachalo_v34_validation?sslmode=disable" {
		t.Fatal("dedicated validation cluster DSN required")
	}
	t.Setenv("NACHALO_COMBAT_RULESET", "experimental_v34")
	db, _ := hexCombatDatabase(t)
	file, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	policy := os.Getenv("NACHALO_V34_BOT_POLICY")
	if policy == "" {
		policy = "range_hold"
	}
	if policy != "range_hold" && policy != "legacy_chase" {
		t.Fatal("unknown diagnostic bot policy")
	}
	n := 1
	if value := os.Getenv("NACHALO_V34_N"); value != "" {
		n, err = strconv.Atoi(value)
		if err != nil || n < 1 || n > 2500 {
			t.Fatal("NACHALO_V34_N must be 1..2500")
		}
	}
	classes := []string{"guardian", "berserker", "ranger", "mystic"}
	pairs := [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 0}, {0, 2}, {1, 3}, {0, 0}, {1, 1}, {2, 2}, {3, 3}}
	catalog, err := repository.EquipmentSetsCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	withJewelry := os.Getenv("NACHALO_V34_JEWELRY") == "1"
	completed := map[string]bool{}
	duelKey := func(arena string, tier int, a, b string, first int, seed int64) string {
		return fmt.Sprintf("%s/%d/%s/%s/%d/%d", arena, tier, a, b, first, seed)
	}
	// Resume bounded diagnostics without replaying already-recorded duels. The
	// previous output remains immutable and the new file contains missing rows only.
	if source := os.Getenv("NACHALO_V34_RESUME_FROM"); source != "" {
		previous, e := os.Open(source)
		if e != nil {
			t.Fatal(e)
		}
		decoder := json.NewDecoder(previous)
		for {
			var row v34ValidationRow
			e = decoder.Decode(&row)
			if e == io.EOF {
				break
			}
			if e != nil {
				previous.Close()
				t.Fatal(e)
			}
			if row.EquipmentModel != "slots_v3_reflex" || row.Jewelry != withJewelry || row.Policy != policy {
				previous.Close()
				t.Fatal("resume cohort mismatch")
			}
			key := duelKey(row.Arena, row.Tier, row.A, row.B, row.First, row.Seed)
			if completed[key] {
				previous.Close()
				t.Fatal("duplicate resume duel")
			}
			completed[key] = true
		}
		previous.Close()
	}

	recipes, err := repository.PublicCraftRecipes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	expectedStats := map[int]repository.EquipmentStats{}
	for tier := 0; tier < 3; tier++ {
		if os.Getenv("NACHALO_V34_PILOT") == "1" && tier > 0 {
			break
		}
		var profiles [4][2]int
		for classIndex, class := range classes {
			for seat := 0; seat < 2; seat++ {
				user := 900 + tier*100 + classIndex*2 + seat
				profiles[classIndex][seat] = user
				if _, err = repository.CreatePlayerProfile(repository.CreatePlayerProfileInput{UserID: user, Name: fmt.Sprintf("v34 %d", user), CharacterType: class}); err != nil {
					t.Fatal(err)
				}
				if tier > 0 {
					character, e := repository.GetSelectedCharacterForUser(user)
					if e != nil {
						t.Fatal(e)
					}
					rarity := "green"
					if tier == 2 {
						rarity = "blue"
					}
					found := false
					for _, set := range catalog {
						if set.HeroClass == class && set.Rarity == rarity {
							found = true
							for _, item := range set.Items {
								granted, e := repository.GrantItemInstanceToUser(user, item.Code, "shop")
								if e != nil {
									t.Fatal(e)
								}
								if e = repository.EquipItemToCharacter(user, character.ID, granted.InstanceID); e != nil {
									t.Fatal(e)
								}
							}
							break
						}
					}
					if withJewelry {
						jewelryCount := 0
						for _, recipe := range recipes {
							if recipe.Category == "jewelry" && recipe.HeroClass == class && recipe.Rarity == rarity {
								granted, e := repository.GrantItemInstanceToUser(user, recipe.Code, "craft")
								if e != nil {
									t.Fatal(e)
								}
								if e = repository.EquipItemToCharacter(user, character.ID, granted.InstanceID); e != nil {
									t.Fatal(e)
								}
								jewelryCount++
							}
						}
						if jewelryCount != 2 {
							t.Fatalf("missing jewelry %s/%s", class, rarity)
						}
					}
					if !found {
						t.Fatalf("missing gear %s/%s", class, rarity)
					}

				}
				character, e := repository.GetSelectedCharacterForUser(user)
				if e != nil {
					t.Fatal(e)
				}
				stats, e := repository.GetCharacterEffectiveStats(character.ID)
				if e != nil {
					t.Fatal(e)
				}
				expectedStats[user] = stats.EffectiveStats
			}
		}
		for _, arena := range []string{"open", "tactical"} {
			if filter := os.Getenv("NACHALO_V34_ARENA"); filter != "" && arena != filter {
				continue
			}
			if os.Getenv("NACHALO_V34_PILOT") == "1" && arena != "open" {
				break
			}
			for pairIndex, pair := range pairs {
				if os.Getenv("NACHALO_V34_SKIP_MIRRORS") == "1" && pair[0] == pair[1] {
					continue
				}
				if os.Getenv("NACHALO_V34_PILOT") == "1" && pairIndex > 0 {
					break
				}
				for first := 0; first < 2; first++ {
					for sample := 0; sample < n; sample++ {
						users := [2]int{profiles[pair[0]][0], profiles[pair[1]][1]}
						seed := int64(34000000 + tier*100000 + pairIndex*10000 + first*5000 + sample)
						if completed[duelKey(arena, tier, classes[pair[0]], classes[pair[1]], first, seed)] {
							continue
						}
						t.Setenv("NACHALO_COMBAT_SEED", strconv.FormatInt(seed, 10))
						id := fmt.Sprintf("v34-%s-%d-%d-%d-%d", arena, tier, pairIndex, first, sample)
						order := []int{users[first], users[1-first]}
						started := time.Now()
						_, e := repository.CreateHexMatch(context.Background(), repository.HexMatchPlan{InstanceID: id, Mode: "hex-test", Teams: []hexmap.Team{{ID: 1, PlayerIDs: []int{users[0]}}, {ID: 2, PlayerIDs: []int{users[1]}}}, TurnOrder: order, Seed: 97})
						if e != nil {
							t.Fatal(e)
						}
						cells, walls := v34Arena(t, db, id, arena, users)
						// Start resources/effects through the production personal-turn path.
						c, e := repository.BeginMatchCommand(context.Background(), id, users[first], "arena-start", "arena-start")
						if e != nil {
							t.Fatal(e)
						}
						if e = c.StartHexPersonalTurnTx(context.Background(), users[first]); e != nil {
							c.Rollback()
							t.Fatal(e)
						}
						body := map[string]any{}
						if _, _, e = c.Commit(context.Background(), &body, false); e != nil {
							t.Fatal(e)
						}
						row := v34ValidationRow{EquipmentModel: "slots_v3_reflex", Jewelry: withJewelry, Policy: policy, Arena: arena, Tier: tier, A: classes[pair[0]], B: classes[pair[1]], First: first, Seed: seed, Winner: -1, Initial: v34State(t, db, id, users)}
						for i, s := range row.Initial {
							wantedReflex := []int{5, 12, 15}[tier]
							if withJewelry {
								wantedReflex = []int{5, 15, 20}[tier]
							}
							expected := expectedStats[s.User]
							if s.MaxHP != expected.MaxHealth || s.Attack != expected.Attack || s.Defense != expected.Defense || s.Reflex != wantedReflex {
								t.Fatalf("invalid actual gear budget seat %d: %+v", i, s)
							}
						}
						for action := 0; action < 600; action++ {
							states := v34State(t, db, id, users)
							if states[0].HP <= 0 || states[1].HP <= 0 {
								if states[0].HP > 0 {
									row.Winner = 0
								} else if states[1].HP > 0 {
									row.Winner = 1
								}
								break
							}
							var active, turn int
							var rngBefore int64
							if e = db.QueryRow(`SELECT active_user_id,turn_number,combat_rng_counter FROM matches WHERE instance_id=$1`, id).Scan(&active, &turn, &rngBefore); e != nil {
								t.Fatal(e)
							}
							row.Turns = turn
							if turn > 60 {
								break
							}
							seat := 0
							if active == users[1] {
								seat = 1
							}
							target := users[1-seat]
							command := fmt.Sprintf("action-%d", action)
							kind := "end"
							_, attackErr := PreviewHexPlayerAttack(context.Background(), id, active, target)
							if policy == "range_hold" && states[seat].AP < 2 {
								_, e = repository.ApplyHexEndTurn(context.Background(), id, active, command, turn, nil)
							} else if attackErr == nil {
								_, e = ApplyHexPlayerAttack(context.Background(), id, active, target, command, nil)
								kind = "attack"
								row.Attacks[seat]++
							} else if states[seat].MP > 0 {
								tx, txErr := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
								if txErr != nil {
									t.Fatal(txErr)
								}
								visible, visErr := repository.HexVisibleCellsTx(context.Background(), tx, id, active)
								tx.Rollback()
								if visErr != nil {
									t.Fatal(visErr)
								}
								distances := v34BotDistances(cells, walls, states[1-seat].Position)
								bestScore := int(^uint(0) >> 1)
								var destination hexgrid.Coord
								found := false
								preferred := 1
								if policy == "range_hold" && states[seat].IsRanged {
									preferred = states[seat].AttackRange
									if preferred > states[seat].SightRange {
										preferred = states[seat].SightRange
									}
								}
								floors := map[hexgrid.Coord]bool{}
								for _, cell := range cells {
									floors[cell.Position] = true
								}
								for _, cell := range cells {
									d, ok := distances[cell.Position]
									if !ok || !visible[cell.Position] || d == 0 || cell.Position == states[seat].Position {
										continue
									}
									route, routeErr := repository.PreviewHexRoute(context.Background(), id, active, cell.Position)
									if routeErr != nil || route.AffordableSteps == 0 {
										continue
									}
									rd := distances[route.ReachablePosition]
									endDistance, _ := hexgrid.Distance(route.ReachablePosition, states[1-seat].Position)
									clear := hexgrid.Visible(route.ReachablePosition, states[1-seat].Position, floors, walls)
									if policy == "range_hold" && endDistance <= preferred && !clear {
										continue
									}
									if rd >= distances[states[seat].Position] && !(policy == "range_hold" && endDistance <= preferred && clear) {
										continue
									}
									score := rd*100 + route.ReachableCost
									if policy == "range_hold" {
										miss := endDistance - preferred
										if miss < 0 {
											miss = -miss
										}
										score = miss*1000 + route.ReachableCost
									}
									if score < bestScore {
										bestScore = score
										destination = cell.Position
										found = true
									}
								}
								if found {
									_, e = repository.ApplyHexMove(context.Background(), id, active, command, destination)
									kind = "move"
								} else {
									_, e = repository.ApplyHexEndTurn(context.Background(), id, active, command, turn, nil)
								}
							} else if states[seat].AP >= 2 {
								_, e = repository.ExchangeHexAP(context.Background(), id, active, command)
								kind = "exchange"
							} else {
								_, e = repository.ApplyHexEndTurn(context.Background(), id, active, command, turn, nil)
							}
							if e != nil {
								t.Fatalf("%s %s: %v", id, kind, e)
							}
							if sample == 0 {
								var rngAfter int64
								var activeAfter int
								if e = db.QueryRow(`SELECT combat_rng_counter,COALESCE(active_user_id,0) FROM matches WHERE instance_id=$1`, id).Scan(&rngAfter, &activeAfter); e != nil {
									t.Fatal(e)
								}
								row.Trace = append(row.Trace, v34ValidationAction{Command: command, Kind: kind, User: active, Round: turn, RNGBefore: rngBefore, RNGAfter: rngAfter, ActiveBefore: active, ActiveAfter: activeAfter, Before: states, State: v34State(t, db, id, users)})
							}
						}
						row.Milliseconds = time.Since(started).Milliseconds()
						if err = encoder.Encode(row); err != nil {
							t.Fatal(err)
						}
						if err = file.Sync(); err != nil {
							t.Fatal(err)
						}
						t.Logf("%s winner=%d turns=%d attacks=%v ms=%d", id, row.Winner, row.Turns, row.Attacks, row.Milliseconds)
						if _, e = db.Exec(`DELETE FROM matches WHERE instance_id=$1`, id); e != nil {
							t.Fatal(e)
						}
					}
				}
			}
		}
	}
}
