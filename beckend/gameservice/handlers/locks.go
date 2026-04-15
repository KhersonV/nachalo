package handlers

import "sync"

var (
	playerLocks   = make(map[int]*sync.Mutex)
	playerLocksMu sync.Mutex
)

// lockPlayer serializes actions for a specific userID to avoid concurrent
// modifications (e.g., double energy spend on rapid clicks).
func lockPlayer(userID int) {
	if userID == 0 {
		return
	}
	playerLocksMu.Lock()
	mu, ok := playerLocks[userID]
	if !ok {
		mu = &sync.Mutex{}
		playerLocks[userID] = mu
	}
	playerLocksMu.Unlock()
	mu.Lock()
}

func unlockPlayer(userID int) {
	if userID == 0 {
		return
	}
	playerLocksMu.Lock()
	mu, ok := playerLocks[userID]
	playerLocksMu.Unlock()
	if ok {
		mu.Unlock()
	}
}
