package core

import (
	"sync"
	"time"
)

type Obj struct {
	Value     interface{}
	ExpiresAt int64
}

var (
	store   map[string]*Obj
	storeMu sync.RWMutex
)

func init() {
	store = make(map[string]*Obj)
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

func Put(k string, obj *Obj) {
	storeMu.Lock()
	defer storeMu.Unlock()
	store[k] = obj
}

func Del(k string) bool {
	storeMu.Lock()
	defer storeMu.Unlock()
	if _, ok := store[k]; !ok {
		return false
	}
	delete(store, k)
	return true
}

func Get(k string) *Obj {
	storeMu.RLock()
	obj, ok := store[k]
	storeMu.RUnlock()
	if !ok {
		return nil
	}

	if obj.ExpiresAt != -1 && obj.ExpiresAt <= time.Now().UnixMilli() {
		storeMu.Lock()
		delete(store, k)
		storeMu.Unlock()
		return nil
	}

	return obj
}

func TTL(k string) int64 {
	storeMu.RLock()
	obj, ok := store[k]
	storeMu.RUnlock()
	if !ok {
		return -2
	}

	if obj.ExpiresAt == -1 {
		return -1
	}

	remainingMs := obj.ExpiresAt - time.Now().UnixMilli()
	if remainingMs <= 0 {
		storeMu.Lock()
		delete(store, k)
		storeMu.Unlock()
		return -2
	}

	return remainingMs / 1000
}
