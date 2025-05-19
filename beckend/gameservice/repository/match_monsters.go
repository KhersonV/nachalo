
// ========================================
// gameservice/repository/match_monsters.go
// ========================================



package repository

import (
    "database/sql"
)

// Структура для работы с match_monsters
type MatchMonster struct {
    InstanceID        string
    MonsterInstanceID int
    RefID, X, Y       int
    Health, MaxHealth int
    Attack, Defense   int
    Speed, Maneuverability, Vision int
    Image             string
}

// Вставка монстра при создании матча
func InsertMatchMonster(m MatchMonster) error {
    _, err := DB.Exec(`
        INSERT INTO match_monsters
            (match_instance_id, monster_ref_id, pos_x, pos_y,
             health, max_health, attack, defense, speed, maneuverability, vision, image)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
    `,
        m.InstanceID, m.RefID, m.X, m.Y,
        m.Health, m.MaxHealth, m.Attack, m.Defense,
        m.Speed, m.Maneuverability, m.Vision, m.Image,
    )
    return err
}

// Поиск монстра по позиции
func GetMatchMonsterAt(instanceID string, x, y int) (*MatchMonster, error) {
    var m MatchMonster
    err := DB.QueryRow(`
        SELECT
          match_instance_id, monster_instance_id, monster_ref_id,
          pos_x, pos_y, health, max_health,
          attack, defense, speed, maneuverability, vision, image
        FROM match_monsters
        WHERE match_instance_id=$1 AND pos_x=$2 AND pos_y=$3
    `, instanceID, x, y).Scan(
        &m.InstanceID, &m.MonsterInstanceID, &m.RefID,
        &m.X, &m.Y, &m.Health, &m.MaxHealth,
        &m.Attack, &m.Defense, &m.Speed, &m.Maneuverability, &m.Vision, &m.Image,
    )
    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    return &m, nil
}

// Обновление здоровья монстра после удара
func UpdateMatchMonsterHealth(instanceID string, monsterInstanceID, newHP int) error {
    _, err := DB.Exec(`
        UPDATE match_monsters
           SET health = $1
         WHERE match_instance_id = $2 AND monster_instance_id = $3
    `, newHP, instanceID, monsterInstanceID)
    return err
}
