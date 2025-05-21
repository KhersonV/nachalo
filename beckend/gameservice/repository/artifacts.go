// ====================================
// gameservice/repository/artifacts.go
// ====================================
package repository

import (
    
    "fmt"
    "time"
   "database/sql"
   "errors"
)

type PersistedArtifact struct {
    ID           int
    UserID       int
    ArtifactType string
    ArtifactID   int
    Description  string
    Image        string
    Rarity       string
    Durability   int
    BaseValue    float64
    NPCPrice     float64
    AcquiredAt   time.Time
    ExpiresAt    *time.Time
}

const (
    insertArtifactQuery = `
        INSERT INTO persisted_artifacts
          (user_id, artifact_type, artifact_id, description, image, rarity, durability, base_value, npc_price, acquired_at, expires_at)
        VALUES
          ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        RETURNING id;
    `

    selectUserArtifactsQuery = `
        SELECT id, user_id, artifact_type, artifact_id, description, image,
               rarity, durability, base_value, npc_price, acquired_at, expires_at
        FROM persisted_artifacts
        WHERE user_id = $1;
    `

    deleteArtifactQuery = `
        DELETE FROM persisted_artifacts
        WHERE id = $1;
    `

    updateArtifactOwnerQuery = `
        UPDATE persisted_artifacts
        SET user_id = $2
        WHERE id = $1;
    `

    updateArtifactQuery = `
        UPDATE persisted_artifacts
        SET artifact_type = $2,
            artifact_id   = $3,
            description   = $4,
            image         = $5,
            rarity        = $6,
            durability    = $7,
            base_value    = $8,
            npc_price     = $9,
            expires_at    = $10
        WHERE id = $1;
    `
)

var ErrNotFound = errors.New("artifact not found")


// GetArtifactByID возвращает один PersistedArtifact по его ID.
func GetArtifactByID(id int) (PersistedArtifact, error) {
	const q = `
		SELECT id, user_id, artifact_type, artifact_id, description, image,
		       rarity, durability, base_value, npc_price,
		       acquired_at, expires_at
		FROM persisted_artifacts
		WHERE id = $1
		LIMIT 1;
	`
	var a PersistedArtifact
	err := DB.QueryRow(q, id).Scan(
		&a.ID, &a.UserID, &a.ArtifactType, &a.ArtifactID,
		&a.Description, &a.Image, &a.Rarity, &a.Durability,
		&a.BaseValue, &a.NPCPrice, &a.AcquiredAt, &a.ExpiresAt,
	)
	if err == sql.ErrNoRows {
		return a, ErrNotFound
	}
	if err != nil {
		return a, fmt.Errorf("GetArtifactByID: %w", err)
	}
	return a, nil
}


// AddPersistedArtifact сохраняет новый артефакт в базу и возвращает его ID.
func AddPersistedArtifact(a PersistedArtifact) (int, error) {
    var id int
    err := DB.QueryRow(
        insertArtifactQuery,
        a.UserID, a.ArtifactType, a.ArtifactID, a.Description, a.Image,
        a.Rarity, a.Durability, a.BaseValue, a.NPCPrice, a.AcquiredAt, a.ExpiresAt,
    ).Scan(&id)
    if err != nil {
        return 0, fmt.Errorf("insert persisted_artifact: %w", err)
    }
    return id, nil
}

// GetUserArtifacts возвращает все артефакты указанного пользователя.
func GetUserArtifacts(userID int) ([]PersistedArtifact, error) {
    rows, err := DB.Query(selectUserArtifactsQuery, userID)
    if err != nil {
        return nil, fmt.Errorf("query user artifacts: %w", err)
    }
    defer rows.Close()

    var artifacts []PersistedArtifact
    for rows.Next() {
        var a PersistedArtifact
        if err := rows.Scan(
            &a.ID, &a.UserID, &a.ArtifactType, &a.ArtifactID, &a.Description, &a.Image,
            &a.Rarity, &a.Durability, &a.BaseValue, &a.NPCPrice, &a.AcquiredAt, &a.ExpiresAt,
        ); err != nil {
            return nil, fmt.Errorf("scan artifact: %w", err)
        }
        artifacts = append(artifacts, a)
    }
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("iterate artifact rows: %w", err)
    }
    return artifacts, nil
}

// DeletePersistedArtifact удаляет артефакт по его ID.
func DeletePersistedArtifact(id int) error {
    if _, err := DB.Exec(deleteArtifactQuery, id); err != nil {
        return fmt.Errorf("delete persisted_artifact: %w", err)
    }
    return nil
}

// TransferPersistedArtifact меняет владельца артефакта.
func TransferPersistedArtifact(id, newUserID int) error {
    if _, err := DB.Exec(updateArtifactOwnerQuery, id, newUserID); err != nil {
        return fmt.Errorf("transfer artifact: %w", err)
    }
    return nil
}

// UpdatePersistedArtifact обновляет данные существующего артефакта (кроме даты создания).
func UpdatePersistedArtifact(a PersistedArtifact) error {
    if _, err := DB.Exec(
        updateArtifactQuery,
        a.ID,
        a.ArtifactType, a.ArtifactID, a.Description, a.Image,
        a.Rarity, a.Durability, a.BaseValue, a.NPCPrice, a.ExpiresAt,
    ); err != nil {
        return fmt.Errorf("update persisted_artifact: %w", err)
    }
    return nil
}
