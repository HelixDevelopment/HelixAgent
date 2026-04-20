package handlers

// putTeamForTest installs a Team directly into the handler's teams
// store — replaces the pre-CONST-029 `handler.teamsMu.Lock(); handler.
// teams[id] = team; handler.teamsMu.Unlock()` idiom that test files
// relied on.
func putTeamForTest(h *EnsembleHandler, id string, team *Team) {
	h.teams.Put(id, team)
}

// getTeamForTest retrieves a Team by ID; replaces direct map reads.
func getTeamForTest(h *EnsembleHandler, id string) (*Team, bool) {
	return h.teams.Get(id)
}
