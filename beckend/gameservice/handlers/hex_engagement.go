package handlers

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"gameservice/internal/hexgrid"
	"gameservice/repository"
)

// Geometry planning only; damage and class effects execute in the existing attack helpers.
func previewHexEngagementTx(ctx context.Context, tx *sql.Tx, instance string, user, target int, kind string) (HexAttackPreview, error) {
	var out HexAttackPreview
	if kind != "player" && kind != "monster" {
		return out, repository.ErrCommandRejected
	}
	s, attackErr := readHexAttackTx(ctx, tx, instance, user, target, false, kind)
	if attackErr != nil && !errors.Is(attackErr, repository.ErrHexBlocked) {
		return out, attackErr
	}
	if s.Player.Stats.Health <= 0 || s.Monster.Stats.Health <= 0 {
		return out, repository.ErrHexBlocked
	}
	visible, err := repository.HexVisibleCellsTx(ctx, tx, instance, user)
	if err != nil {
		return out, err
	}
	if !visible[s.Monster.Position] {
		return out, repository.ErrHexBlocked
	}
	out = HexAttackPreview{TargetID: target, Distance: s.Distance, AP: s.Player.AP, TargetHP: s.Monster.Stats.Health, Mode: s.Mode, AttackPosition: s.Player.Position, PlannedTargetPosition: s.Monster.Position}
	if out.Mode == "" {
		out.Mode = attackModeMelee
		if s.Player.Stats.IsRanged {
			out.Mode = attackModeRanged
		}
	}
	out.Cost = resolveAttackEnergyCost(out.Mode)
	if err = tx.QueryRowContext(ctx, `SELECT state_revision,map_revision FROM matches WHERE instance_id=$1`, instance).Scan(&out.PreviewRevision, &out.PreviewMapRevision); err != nil {
		return out, err
	}
	if out.AP < out.Cost {
		out.Reason = "Недостаточно AP для атаки"
	} else if attackErr == nil {
		out.CanAttack = true
	} else {
		reach := 1
		if s.Player.Stats.IsRanged && s.Player.Stats.AttackRange > reach {
			reach = s.Player.Stats.AttackRange
		}
		// Iterate only known cells, instead of querying/probing any hidden destination.
		candidates := make([]hexgrid.Coord, 0, len(visible))
		for p := range visible {
			d, e := hexgrid.Distance(p, s.Monster.Position)
			if e == nil && d >= 1 && d <= reach && p != s.Player.Position {
				candidates = append(candidates, p)
			}
		}
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].Q != candidates[j].Q {
				return candidates[i].Q < candidates[j].Q
			}
			return candidates[i].R < candidates[j].R
		})
		for _, p := range candidates {
			at, e := readHexAttackFromTx(ctx, tx, instance, user, target, false, &p, kind)
			if e != nil {
				if errors.Is(e, repository.ErrHexBlocked) {
					continue
				}
				return out, e
			}
			route, e := repository.FindHexRouteTx(ctx, tx, instance, user, p)
			if e != nil {
				if errors.Is(e, repository.ErrHexBlocked) {
					continue
				}
				return out, e
			}
			if out.Route == nil || route.TotalCost < out.Route.TotalCost || route.TotalCost == out.Route.TotalCost && len(route.Path) < len(out.Route.Path) {
				copy := route
				out.Route = &copy
				out.AttackPosition = p
				out.Mode, out.Cost = at.Mode, at.Cost
			}
		}
		if out.Route == nil {
			out.Reason = "Нет доступного видимого маршрута к позиции атаки"
		} else if out.Route.Affordable {
			out.CanAttack = true
		} else if out.Route.AffordableSteps > 0 {
			out.Reason = "Доступно частичное сближение; для достижения позиции атаки недостаточно MP"
		} else {
			out.Reason = "Недостаточно MP даже для первого шага"
		}
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return out, err
	}
	digest := sha256.Sum256(raw)
	out.PreviewToken = hex.EncodeToString(digest[:])
	return out, nil
}

func PreviewHexEngagement(ctx context.Context, instance string, user, target int, kind string) (HexAttackPreview, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	tx, err := repository.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return HexAttackPreview{}, err
	}
	defer tx.Rollback()
	out, err := previewHexEngagementTx(ctx, tx, instance, user, target, kind)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}

type HexEngagementResult struct {
	HexAttackResult
	Movement *repository.HexRouteResult `json:"movement,omitempty"`
	Attacked bool                       `json:"attacked"`
	Reason   string                     `json:"reason,omitempty"`
}

func (r *HexEngagementResult) SetCommandRevisions(state, revision int64) {
	r.HexAttackResult.SetCommandRevisions(state, revision)
	if r.Movement != nil {
		r.Movement.SetCommandRevisions(state, revision)
	}
}

func ApplyHexEngagement(ctx context.Context, instance string, user, target int, kind, commandID, token string, publish func(HexEngagementResult)) (HexEngagementResult, error) {
	var out HexEngagementResult
	if token == "" {
		return out, repository.ErrCommandRejected
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	c, err := repository.BeginMatchCommand(ctx, instance, user, commandID, fmt.Sprintf("hex-engage-v1:%d:%s:%d:%s", user, kind, target, token))
	if err != nil {
		return out, err
	}
	defer c.Rollback()
	if c.Replay != nil {
		err = json.Unmarshal(c.Replay, &out)
		out.Replayed = true
		return out, err
	}
	planned, err := previewHexEngagementTx(ctx, c.Tx, instance, user, target, kind)
	if err != nil {
		return out, err
	}
	if planned.PreviewToken != token {
		return out, repository.ErrCommandRejected
	}
	if planned.AP < planned.Cost {
		return out, repository.ErrHexEnergy
	}
	if !planned.CanAttack && (planned.Route == nil || planned.Route.AffordableSteps == 0) {
		return out, repository.ErrHexBlocked
	}
	out.HexAttackPreview = planned
	out.NewTargetHP = planned.TargetHP
	if kind == "player" {
		c.CombatTargetKind, c.CombatTargetID = kind, target
	}
	if planned.Route != nil {
		moved, e := repository.ApplyHexMoveTx(ctx, c, user, planned.AttackPosition)
		if e != nil {
			return out, e
		}
		out.Movement = &moved
	}
	var hp int
	if err = c.Tx.QueryRowContext(ctx, `SELECT health FROM match_players WHERE instance_id=$1 AND user_id=$2`, instance, user).Scan(&hp); err != nil {
		return out, err
	}
	out.NewAttackerHP = hp
	// Gameplay death during a route is a successful action; technical errors still roll back everything.
	if hp > 0 && (out.Movement == nil || !out.Movement.Partial) {
		query := `SELECT health FROM match_players WHERE instance_id=$1 AND user_id=$2`
		if kind == "monster" {
			query = `SELECT health FROM match_monsters WHERE instance_id=$1 AND monster_instance_id=$2`
		}
		var targetHP int
		err = c.Tx.QueryRowContext(ctx, query, instance, target).Scan(&targetHP)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return out, err
		}
		if err == nil && targetHP > 0 {
			before, e := c.AuraMembersTx(ctx)
			if e != nil {
				return out, e
			}
			var hit HexAttackResult
			if kind == "player" {
				hit, err = applyHexPlayerAttackTx(ctx, c, user, target)
			} else {
				hit, err = applyHexAttackTx(ctx, c, user, target)
			}
			if err != nil {
				return out, err
			}
			out.HexAttackResult, out.Attacked = hit, true
			if err = c.ResolveAuraTransitionTx(ctx, before); err != nil {
				return out, err
			}
			c.AuraResolved = true
		} else {
			out.NewTargetHP = 0
			out.Reason = "Цель погибла во время сближения"
		}
	} else if hp <= 0 {
		out.Reason = "Герой погиб во время сближения"
	} else {
		out.Reason = planned.Reason
	}
	out.StateRevision, out.MapRevision, err = c.Commit(ctx, &out, out.Movement != nil || c.MapChanged)
	if err != nil {
		return HexEngagementResult{}, err
	}
	if publish != nil {
		publish(out)
	}
	return out, nil
}
