package repository

import (
	"context"
	"database/sql"

	"gameservice/internal/hexgrid"
)

const (
	hexGuardianAuraRange     = 2 // Square guardianZoneControlRange
	hexGuardianMovePenalty   = 1 // Square guardianZoneControlMovePenalty
	hexGuardianExitDamageCap = 5 // Square guardianAuraExitDamageCap
)

type hexGuardian struct {
	userID, teamID int
	position       hexgrid.Coord
}

type hexAuraPressure struct{ amount, sourceUserID int }
type hexAuraHit struct{ step, damage, sourceUserID int }

func loadHexGuardiansTx(ctx context.Context, tx *sql.Tx, instanceID string) ([]hexGuardian, error) {
	rows, err := tx.QueryContext(ctx, `SELECT user_id,COALESCE(group_id,0),position_q,position_r FROM match_players WHERE instance_id=$1 AND character_type='guardian' AND health>0 ORDER BY user_id`, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	guardians := []hexGuardian{}
	for rows.Next() {
		var g hexGuardian
		if err := rows.Scan(&g.userID, &g.teamID, &g.position.Q, &g.position.R); err != nil {
			return nil, err
		}
		guardians = append(guardians, g)
	}
	return guardians, rows.Err()
}

func hexAuraSourceAt(pos hexgrid.Coord, userID, teamID int, guardians []hexGuardian) int {
	source := 0
	for _, g := range guardians {
		if g.userID == userID || (teamID != 0 && teamID == g.teamID) {
			continue
		}
		distance, err := hexgrid.Distance(pos, g.position)
		if err == nil && distance <= hexGuardianAuraRange && (source == 0 || g.userID < source) {
			source = g.userID
		}
	}
	return source
}

// A HEX route is several Square-sized moves. Charge every step begun inside
// the aura and discharge accumulated pressure only on a voluntary exit.
func planHexGuardianAura(start hexgrid.Coord, path []hexgrid.Coord, userID, teamID int, guardians []hexGuardian, pressure hexAuraPressure) (int, hexAuraPressure, []hexAuraHit) {
	extra := 0
	hits := []hexAuraHit{}
	from := start
	for i, to := range path {
		source := hexAuraSourceAt(from, userID, teamID, guardians)
		if source == 0 {
			pressure = hexAuraPressure{}
		}
		if source != 0 {
			extra += hexGuardianMovePenalty
			pressure.amount += hexGuardianMovePenalty
			pressure.sourceUserID = source
			if hexAuraSourceAt(to, userID, teamID, guardians) == 0 {
				damage := pressure.amount
				if damage > hexGuardianExitDamageCap {
					damage = hexGuardianExitDamageCap
				}
				if damage > 0 {
					hits = append(hits, hexAuraHit{step: i, damage: damage, sourceUserID: pressure.sourceUserID})
				}
				pressure = hexAuraPressure{}
			}
		}
		from = to
	}
	return extra, pressure, hits
}

// Forced displacement never deals exit damage, but leaving all auras clears pressure.
func ClearHexGuardianAuraAfterForcedMoveTx(ctx context.Context, tx *sql.Tx, instanceID string, userID int, from, destination hexgrid.Coord) error { return nil }
