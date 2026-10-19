package handlers

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"gameservice/repository"
)

var hexTimerMu sync.Mutex
var hexTimers = map[string]struct {
	turn   int
	cancel context.CancelFunc
}{}

// Each process may schedule this timer. PostgreSQL's turn check ensures only
// the first expiry or player action can advance the turn. On restart GET rearms
// the timer for the current snapshot; a durable deadline is still future work.
func scheduleHexTurn(instanceID string, userID, turn int) {
	if instanceID == "" || userID <= 0 || turn < 1 {
		return
	}
	hexTimerMu.Lock()
	if current, ok := hexTimers[instanceID]; ok {
		if current.turn == turn {
			hexTimerMu.Unlock()
			return
		}
		current.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	hexTimers[instanceID] = struct {
		turn   int
		cancel context.CancelFunc
	}{turn, cancel}
	hexTimerMu.Unlock()
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(turnTimerLimit):
		}
		hexTimerMu.Lock()
		if current,ok:=hexTimers[instanceID]; ok && current.turn==turn { delete(hexTimers,instanceID) }
		hexTimerMu.Unlock()
		commandID := fmt.Sprintf("hex-timer:%d", turn)
		result, err := repository.ApplyHexEndTurn(context.Background(), instanceID, userID, commandID, turn, resolveCharacterRegen, func(committed repository.HexTurnResult) {
			publishHexPatch(instanceID, commandID, committed.StateRevision, committed.MapRevision, "TURN_PASSED", map[string]any{"instanceId": instanceID, "userId": committed.ActiveUser, "active_user": committed.ActiveUser, "turnNumber": committed.TurnNumber, "energy": committed.Energy})
		})
		if err != nil && err != repository.ErrCommandRejected {
			log.Printf("HEX timer %s turn %d: %v", instanceID, turn, err)
		}
		if err == nil && !result.Replayed {
			scheduleHexTurn(instanceID, userID, result.TurnNumber)
		}
	}()
}
