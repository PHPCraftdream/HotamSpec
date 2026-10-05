package diagnose

// IsBlockingHit is true only for an explicit unresolved Conflict carrier whose
// member IDs identify the candidate and matched requirement. Lexical markers,
// token overlap, source links, and other relations remain suspicion evidence;
// none alone proves a semantic contradiction.
func IsBlockingHit(h ConfrontHit) bool {
	return h.Classification == ClassificationFormalConflict && len(h.ConflictIDs) > 0
}
