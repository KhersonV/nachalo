package repository

import "context"

func (c *MatchCommand) SetCombatPlayerVitalsTx(ctx context.Context, userID, health, energy int) error {
	if c == nil || c.Tx == nil {
		return ErrCommandRejected
	}
	result, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET health=$3,energy=$4 WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, userID, health, energy)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrCommandRejected
	}
	return nil
}

func (c *MatchCommand) SetMonsterHealthTx(ctx context.Context, monsterID, health int) error {
	if c == nil || c.Tx == nil {
		return ErrCommandRejected
	}
	result, err := c.Tx.ExecContext(ctx, `UPDATE match_monsters SET health=$3 WHERE instance_id=$1 AND monster_instance_id=$2`, c.InstanceID, monsterID, health)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrCommandRejected
	}
	return nil
}
