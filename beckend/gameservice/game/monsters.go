
// =============================
// gameservice/game/monsters.go
// =============================



package game

// Вызывается при создании матча, после InsertMatchMonster для всех монстров БД.

func (ms *MatchState) InitMonstersFromCells(cells []FullCell) {
    ms.Monsters = make(map[int]*MonsterState)
    for _, c := range cells {
        if c.Monster != nil && c.Monster.DBInstanceID != 0 {
            ms.Monsters[c.Monster.DBInstanceID] = &MonsterState{
                ID:                c.Monster.DBInstanceID,
                MonsterInstanceID: c.Monster.DBInstanceID,
                Health:            c.Monster.Health,
                MaxHealth:         c.Monster.MaxHealth,
                Attack:            c.Monster.Attack,
                Defense:           c.Monster.Defense,
                Speed:             c.Monster.Speed,
                Maneuverability:   c.Monster.Maneuverability,
                Vision:            c.Monster.Vision,
                Image:             c.Monster.Image,
            }
        }
    }
}

