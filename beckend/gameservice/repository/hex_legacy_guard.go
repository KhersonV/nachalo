package repository

// Legacy square item and object handlers are not transaction-safe for HEX.
func IsHexMatch(instanceID string) (bool, error) {
	var kind string
	err := DB.QueryRow(`SELECT COALESCE(geometry->>'kind','') FROM matches WHERE instance_id=$1`, instanceID).Scan(&kind)
	return kind == "hex", err
}
