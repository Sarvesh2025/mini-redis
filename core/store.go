package core

import (
	"sync"
	"time"

	"mini-redis/config"
)

type Obj struct {
	Value     interface{}
	ExpiresAt int64
}

var (
	store       map[string]*Obj
	keyVersions map[string]uint64
	storeMu     sync.RWMutex
)

func init() {
	store = make(map[string]*Obj)
	keyVersions = make(map[string]uint64)
}

func NewObj(value interface{}, durationMs int64) *Obj {
	expiresAt := int64(-1)
	if durationMs > 0 {
		expiresAt = time.Now().UnixMilli() + durationMs
	}

	return &Obj{
		Value:     value,
		ExpiresAt: expiresAt,
	}
}

// --- Unlocked operations (caller must hold storeMu) ---

func put(k string, obj *Obj) {
	if len(store) >= config.KeysLimit {
		evict()
	}
	store[k] = obj
	keyVersions[k]++
}

func get(k string) *Obj {
	obj, ok := store[k]
	if !ok {
		return nil
	}
	if obj.ExpiresAt != -1 && obj.ExpiresAt <= time.Now().UnixMilli() {
		delete(store, k)
		keyVersions[k]++
		return nil
	}
	return obj
}

func del(k string) bool {
	if _, ok := store[k]; !ok {
		return false
	}
	delete(store, k)
	keyVersions[k]++
	return true
}

func getTTL(k string) int64 {
	obj, ok := store[k]
	if !ok {
		return -2
	}
	if obj.ExpiresAt == -1 {
		return -1
	}
	remainingMs := obj.ExpiresAt - time.Now().UnixMilli()
	if remainingMs <= 0 {
		delete(store, k)
		keyVersions[k]++
		return -2
	}
	return remainingMs / 1000
}

func setExpire(k string, expiresAt int64) bool {
	obj, ok := store[k]
	if !ok {
		return false
	}
	if obj.ExpiresAt != -1 && obj.ExpiresAt <= time.Now().UnixMilli() {
		delete(store, k)
		keyVersions[k]++
		return false
	}
	obj.ExpiresAt = expiresAt
	keyVersions[k]++
	return true
}

func keyVersion(k string) uint64 {
	return keyVersions[k]
}

// --- Locked operations (safe for single-command use) ---

func Put(k string, obj *Obj) {
	storeMu.Lock()
	defer storeMu.Unlock()
	put(k, obj)
}

func Get(k string) *Obj {
	storeMu.Lock()
	defer storeMu.Unlock()
	return get(k)
}

func Del(k string) bool {
	storeMu.Lock()
	defer storeMu.Unlock()
	return del(k)
}

func TTL(k string) int64 {
	storeMu.Lock()
	defer storeMu.Unlock()
	return getTTL(k)
}

func SetExpire(k string, expiresAt int64) bool {
	storeMu.Lock()
	defer storeMu.Unlock()
	return setExpire(k, expiresAt)
}

func KeyVersion(k string) uint64 {
	storeMu.RLock()
	defer storeMu.RUnlock()
	return keyVersion(k)
}
