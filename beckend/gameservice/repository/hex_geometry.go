package repository

// IsHexMatch checks persisted geometry without loading a match snapshot.
func IsHexMatch(instanceID string) (bool, error) {
	var kind string
	err := DB.QueryRow(`SELECT COALESCE(geometry->>'kind','') FROM matches WHERE instance_id=$1`, instanceID).Scan(&kind)
	return kind == "hex", err
}
