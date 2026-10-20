package repository

import (
	"context"
	"os"
	"strconv"
	"time"
)

func (c *MatchCommand) ResetHexTurnDeadlineTx(ctx context.Context, activeUser int) error {
	if activeUser == 0 {
		_, err := c.Tx.ExecContext(ctx, `UPDATE matches SET hex_turn_deadline=NULL WHERE instance_id=$1`, c.InstanceID)
		return err
	}
	_, err := c.Tx.ExecContext(ctx, `UPDATE matches SET hex_turn_deadline=now()+$2 * interval '1 second' WHERE instance_id=$1`, c.InstanceID, int(HexTurnDuration().Seconds()))
	return err
}

func HexTurnDuration() time.Duration {
	if n, err := strconv.Atoi(os.Getenv("HEX_TURN_SECONDS")); err == nil && n >= 5 && n <= 3600 {
		return time.Duration(n) * time.Second
	}
	return 60 * time.Second
}
