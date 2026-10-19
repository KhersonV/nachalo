package repository

import (
	"context"
	"encoding/json"

	"gameservice/game"
)

// LoadCombatResultEventsTx reads the durable input to match-result calculation.
// It uses the same matches row already locked by BeginMatchCommand.
func (c *MatchCommand) LoadCombatResultEventsTx(ctx context.Context) (game.ResultEvents, error) {
	var events game.ResultEvents
	if c == nil || c.Tx == nil {
		return events, ErrCommandRejected
	}
	var raw []byte
	if err := c.Tx.QueryRowContext(ctx, `SELECT combat_result_events FROM matches WHERE instance_id=$1`, c.InstanceID).Scan(&raw); err != nil {
		return events, err
	}
	if err := json.Unmarshal(raw, &events); err != nil {
		return events, err
	}
	return events, nil
}

func (c *MatchCommand) appendCombatResultEventTx(ctx context.Context, field string, value interface{}) error {
	if c == nil || c.Tx == nil {
		return ErrCommandRejected
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = c.Tx.ExecContext(ctx, `UPDATE matches SET combat_result_events=jsonb_set(combat_result_events,ARRAY[$2],COALESCE(combat_result_events->$2,'[]'::jsonb) || $3::jsonb) WHERE instance_id=$1`, c.InstanceID, field, encoded)
	return err
}

func (c *MatchCommand) RecordDamageEventTx(ctx context.Context, event game.DamageEvent) error {
	if event.Amount <= 0 {
		return nil
	}
	return c.appendCombatResultEventTx(ctx, "damageEvents", event)
}

func (c *MatchCommand) RecordKillEventTx(ctx context.Context, event game.KillEvent) (bool, error) {
	events, err := c.LoadCombatResultEventsTx(ctx)
	if err != nil {
		return false, err
	}
	for _, recorded := range events.KillEvents {
		if event.VictimID > 0 && recorded.VictimType == event.VictimType && recorded.VictimID == event.VictimID {
			return false, nil
		}
	}
	if event.VictimType == "player" {
		for _, defeated := range events.DefeatedUsers {
			if defeated == event.VictimID {
				return false, nil
			}
		}
	}
	return true, c.appendCombatResultEventTx(ctx, "killEvents", event)
}

func (c *MatchCommand) RecordDefeatedUserTx(ctx context.Context, userID int) (bool, error) {
	events, err := c.LoadCombatResultEventsTx(ctx)
	if err != nil {
		return false, err
	}
	for _, defeated := range events.DefeatedUsers {
		if defeated == userID {
			return false, nil
		}
	}
	return true, c.appendCombatResultEventTx(ctx, "defeatedUsers", userID)
}
