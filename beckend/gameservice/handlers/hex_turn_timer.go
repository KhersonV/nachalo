package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"gameservice/repository"
)

// PostgreSQL owns the deadline. Concurrent workers race safely on the match
// row lock; only the worker that still sees an expired deadline advances it.
func StartHexTurnWorker(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				expireHexTurns(ctx)
				expireHexDisconnects(ctx)
			}
		}
	}()
}

func expireHexDisconnects(ctx context.Context) {
	rows, err := repository.DB.QueryContext(ctx, `SELECT p.instance_id,p.user_id FROM match_players p JOIN matches m ON m.instance_id=p.instance_id WHERE m.geometry->>'kind'='hex' AND m.finished_at IS NULL AND p.health>0 AND m.turn_order @> to_jsonb(p.user_id) AND p.hex_disconnected_at<=now()-$1 * interval '1 second' ORDER BY p.hex_disconnected_at LIMIT 100`, int(repository.HexReconnectGrace.Seconds()))
	if err != nil {
		log.Printf("HEX disconnect scan: %v", err)
		return
	}
	type due struct {
		id   string
		user int
	}
	var all []due
	for rows.Next() {
		var d due
		if err := rows.Scan(&d.id, &d.user); err != nil {
			log.Printf("HEX disconnect scan: %v", err)
			break
		}
		all = append(all, d)
	}
	if err := rows.Err(); err != nil {
		log.Printf("HEX disconnect scan: %v", err)
	}
	rows.Close()
	for _, d := range all {
		_, err := repository.ApplyHexLightning(ctx, d.id, d.user, func(result repository.HexLightningResult) {
			publishHexPatch(d.id, fmt.Sprintf("hex-lightning:%d", d.user), result.StateRevision, result.MapRevision, "PLAYER_LIGHTNING", map[string]any{"userId": d.user})
		})
		if err != nil && err != repository.ErrCommandRejected {
			log.Printf("HEX lightning %s user %d: %v", d.id, d.user, err)
		}
	}
}

func expireHexTurns(ctx context.Context) {
	rows, err := repository.DB.QueryContext(ctx, `SELECT instance_id,active_user_id,turn_number FROM matches WHERE geometry->>'kind'='hex' AND finished_at IS NULL AND hex_turn_deadline<=now() AND active_user_id IS NOT NULL ORDER BY hex_turn_deadline LIMIT 100`)
	if err != nil {
		log.Printf("HEX turn scan: %v", err)
		return
	}
	type due struct {
		id         string
		user, turn int
	}
	all := []due{}
	for rows.Next() {
		var d due
		if err := rows.Scan(&d.id, &d.user, &d.turn); err != nil {
			rows.Close()
			log.Printf("HEX turn scan: %v", err)
			return
		}
		all = append(all, d)
	}
	if err := rows.Err(); err != nil {
		log.Printf("HEX turn scan: %v", err)
	}
	rows.Close()
	for _, d := range all {
		commandID := fmt.Sprintf("hex-timeout:%d:%d", d.turn, d.user)
		_, err := repository.ApplyHexExpiredTurn(ctx, d.id, d.user, commandID, d.turn, resolveCharacterRegen, func(result repository.HexTurnResult) {
			publishHexPatch(d.id, commandID, result.StateRevision, result.MapRevision, "TURN_TIMEOUT", nil)
		})
		if err != nil && err != repository.ErrCommandRejected {
			log.Printf("HEX timeout %s turn %d: %v", d.id, d.turn, err)
		}
	}
}

// Existing call sites retain these no-op hooks; they never reset a deadline.
func scheduleHexTurn(string, int, int) {}
func cancelHexTurn(string)             {}
