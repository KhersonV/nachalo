package repository

import (
	"context"
	"database/sql/driver"
	"fmt"
	"time"
)

// AcquireMatchSerialization holds a PostgreSQL session advisory lock across a
// legacy command. It coordinates HTTP commands and timers across Go processes.
// It does NOT make the enclosed writes atomic; migrated commands must still
// use MatchCommand for their transaction and ledger.
func AcquireMatchSerialization(ctx context.Context, instanceID string) (func(), error) {
	if DB == nil || instanceID == "" {
		return nil, ErrCommandRejected
	}
	conn, err := DB.Conn(ctx)
	if err != nil {
		return nil, err
	}
	// Two-key namespace avoids collisions with unrelated advisory lock users.
	var ignored interface{}
	if err := conn.QueryRowContext(ctx, `SELECT pg_advisory_lock(735219, hashtext($1))`, instanceID).Scan(&ignored); err != nil {
		conn.Close()
		return nil, fmt.Errorf("lock match %s: %w", instanceID, err)
	}
	return func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var released bool
		if err := conn.QueryRowContext(unlockCtx, `SELECT pg_advisory_unlock(735219, hashtext($1))`, instanceID).Scan(&released); err != nil || !released {
			// A failed unlock must not return a locked session to the pool.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		_ = conn.Close()
	}, nil
}

func MatchTurnForSerialization(instanceID string) (int, int, error) {
	var active, turn int
	err := DB.QueryRow(`SELECT active_user_id,turn_number FROM matches WHERE instance_id=$1`, instanceID).Scan(&active, &turn)
	return active, turn, err
}
