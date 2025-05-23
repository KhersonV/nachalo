// ================================
// /gameservice/repository/utils.go
// ================================

package repository

// CollisionCount возвращает число других игроков в клетке (x,y)
func CollisionCount(instanceID string, x, y, selfID int) (int, error) {
	const q = `
		SELECT COUNT(*) FROM match_players
		WHERE instance_id = $1
		  AND (position->>'x')::int = $2
		  AND (position->>'y')::int = $3
		  AND user_id <> $4
		  AND health > 0
	`
	var cnt int
	err := DB.QueryRow(q, instanceID, x, y, selfID).Scan(&cnt)
	return cnt, err
}

// GetOtherPlayerID возвращает user_id другого игрока в клетке (x,y)
func GetOtherPlayerID(instanceID string, x, y, selfID int) (int, error) {
	const q = `
		SELECT user_id FROM match_players
		WHERE instance_id = $1
		  AND (position->>'x')::int = $2
		  AND (position->>'y')::int = $3
		  AND user_id <> $4
		LIMIT 1
	`
	var otherID int
	err := DB.QueryRow(q, instanceID, x, y, selfID).Scan(&otherID)
	return otherID, err
}
