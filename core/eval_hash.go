package core

import (
	"errors"
)

func evalHSET(args []string) []byte {
	if len(args) < 3 || len(args[1:])%2 != 0 {
		return Encode(errors.New("ERR wrong number of arguments for 'hset' command"), false)
	}

	storeMu.Lock()
	defer storeMu.Unlock()

	return hset(args[0], args[1:])
}

func evalHGET(args []string) []byte {
	if len(args) != 2 {
		return Encode(errors.New("ERR wrong number of arguments for 'hget' command"), false)
	}

	storeMu.RLock()
	defer storeMu.RUnlock()

	return hget(args[0], args[1])
}

func evalHDEL(args []string) []byte {
	if len(args) < 2 {
		return Encode(errors.New("ERR wrong number of arguments for 'hdel' command"), false)
	}

	storeMu.Lock()
	defer storeMu.Unlock()

	return hdel(args[0], args[1:])
}

func evalHGETALL(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'hgetall' command"), false)
	}

	storeMu.RLock()
	defer storeMu.RUnlock()

	return hgetall(args[0])
}

func evalHLEN(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'hlen' command"), false)
	}

	storeMu.RLock()
	defer storeMu.RUnlock()

	return hlen(args[0])
}

func evalHEXISTS(args []string) []byte {
	if len(args) != 2 {
		return Encode(errors.New("ERR wrong number of arguments for 'hexists' command"), false)
	}

	storeMu.RLock()
	defer storeMu.RUnlock()

	return hexists(args[0], args[1])
}

// --- Internal variants (caller holds storeMu) ---

func evalHSETInternal(args []string) []byte {
	if len(args) < 3 || len(args[1:])%2 != 0 {
		return Encode(errors.New("ERR wrong number of arguments for 'hset' command"), false)
	}
	return hset(args[0], args[1:])
}

func evalHGETInternal(args []string) []byte {
	if len(args) != 2 {
		return Encode(errors.New("ERR wrong number of arguments for 'hget' command"), false)
	}
	return hget(args[0], args[1])
}

func evalHDELInternal(args []string) []byte {
	if len(args) < 2 {
		return Encode(errors.New("ERR wrong number of arguments for 'hdel' command"), false)
	}
	return hdel(args[0], args[1:])
}

func evalHGETALLInternal(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'hgetall' command"), false)
	}
	return hgetall(args[0])
}

func evalHLENInternal(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'hlen' command"), false)
	}
	return hlen(args[0])
}

func evalHEXISTSInternal(args []string) []byte {
	if len(args) != 2 {
		return Encode(errors.New("ERR wrong number of arguments for 'hexists' command"), false)
	}
	return hexists(args[0], args[1])
}

// --- Core hash operations (caller must hold storeMu) ---

func hset(key string, fieldValues []string) []byte {
	obj := get(key)
	var hash map[string]string
	var added int64

	if obj == nil {
		hash = make(map[string]string)
		for i := 0; i < len(fieldValues); i += 2 {
			hash[fieldValues[i]] = fieldValues[i+1]
			added++
		}
		put(key, NewObj(hash, -1))
		return Encode(added, false)
	}

	hash, ok := obj.Value.(map[string]string)
	if !ok {
		return Encode(errors.New("WRONGTYPE Operation against a key holding the wrong kind of value"), false)
	}

	for i := 0; i < len(fieldValues); i += 2 {
		if _, exists := hash[fieldValues[i]]; !exists {
			added++
		}
		hash[fieldValues[i]] = fieldValues[i+1]
	}
	obj.Value = hash
	keyVersions[key]++
	return Encode(added, false)
}

func hget(key, field string) []byte {
	obj := get(key)
	if obj == nil {
		return RESP_NIL
	}

	hash, ok := obj.Value.(map[string]string)
	if !ok {
		return Encode(errors.New("WRONGTYPE Operation against a key holding the wrong kind of value"), false)
	}

	val, exists := hash[field]
	if !exists {
		return RESP_NIL
	}
	return Encode(val, false)
}

func hdel(key string, fields []string) []byte {
	obj := get(key)
	if obj == nil {
		return RESP_ZERO
	}

	hash, ok := obj.Value.(map[string]string)
	if !ok {
		return Encode(errors.New("WRONGTYPE Operation against a key holding the wrong kind of value"), false)
	}

	var deleted int64
	for _, field := range fields {
		if _, exists := hash[field]; exists {
			delete(hash, field)
			deleted++
		}
	}

	if len(hash) == 0 {
		del(key)
	} else {
		keyVersions[key]++
	}

	return Encode(deleted, false)
}

func hgetall(key string) []byte {
	obj := get(key)
	if obj == nil {
		return RESP_EMPTY_ARRAY
	}

	hash, ok := obj.Value.(map[string]string)
	if !ok {
		return Encode(errors.New("WRONGTYPE Operation against a key holding the wrong kind of value"), false)
	}

	result := make([]string, 0, len(hash)*2)
	for field, val := range hash {
		result = append(result, field, val)
	}
	return EncodeBulkStringArray(result)
}

func hlen(key string) []byte {
	obj := get(key)
	if obj == nil {
		return RESP_ZERO
	}

	hash, ok := obj.Value.(map[string]string)
	if !ok {
		return Encode(errors.New("WRONGTYPE Operation against a key holding the wrong kind of value"), false)
	}

	return Encode(int64(len(hash)), false)
}

func hexists(key, field string) []byte {
	obj := get(key)
	if obj == nil {
		return RESP_ZERO
	}

	hash, ok := obj.Value.(map[string]string)
	if !ok {
		return Encode(errors.New("WRONGTYPE Operation against a key holding the wrong kind of value"), false)
	}

	if _, exists := hash[field]; exists {
		return RESP_ONE
	}
	return RESP_ZERO
}
