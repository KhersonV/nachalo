package repository

import (
	"context"
	"database/sql"
	"errors"
)

// IsHexTestMatch is a read-only geometry guard for legacy square handlers.
// Match mode is immutable after creation; HEX writes still validate it again
// inside their MatchCommand transaction.
func IsHexTestMatch(ctx context.Context, instanceID string) (bool, error) {
	if DB == nil {
		return false, nil
	}
	var mode string
	if err := DB.QueryRowContext(ctx, `SELECT CASE WHEN COALESCE(to_jsonb(matches)->'geometry'->>'kind','')='hex' OR COALESCE(to_jsonb(matches)->>'mode','')='hex-test' THEN 'hex' ELSE '' END FROM matches WHERE instance_id=$1`, instanceID).Scan(&mode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return mode == "hex", nil
}
