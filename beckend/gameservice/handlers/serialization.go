package handlers

import "gameservice/repository"

// Replaced only by the handler unit tests, which do not open PostgreSQL.
var acquireMatchSerialization = repository.AcquireMatchSerialization
var matchTurnForSerialization = repository.MatchTurnForSerialization
