package core

import (
	"log"
	"time"
)

func expireSample() float32 {
	limit := 20
	expiredCount := 0

	storeMu.Lock()
	defer storeMu.Unlock()

	for key, obj := range store {
		if obj.ExpiresAt != -1 {
			limit--
			if obj.ExpiresAt <= time.Now().UnixMilli() {
				delete(store, key)
				expiredCount++
			}
		}

		if limit == 0 {
			break
		}
	}

	return float32(expiredCount) / float32(20)
}

func DeleteExpiredKeys() {
	for {
		if frac := expireSample(); frac < 0.25 {
			break
		}
	}
	storeMu.RLock()
	total := len(store)
	storeMu.RUnlock()
	log.Println("active-expire cycle complete. total keys:", total)
}
