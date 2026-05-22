package core

// TODO: Make it efficient by doing thorough sampling.
func evictFirst() {
	for k := range store {
		delete(store, k)
		keyVersions[k]++
		return
	}
}

// TODO: Make the eviction strategy configuration driven.
// TODO: Support multiple eviction strategies (allkeys-lru, allkeys-random, etc.).
func evict() {
	evictFirst()
}
