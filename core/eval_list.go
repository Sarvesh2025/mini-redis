package core

import (
	"errors"
	"strconv"
)

var RESP_EMPTY_ARRAY = []byte("*0\r\n")

func evalLPUSH(args []string) []byte {
	if len(args) < 2 {
		return Encode(errors.New("ERR wrong number of arguments for 'lpush' command"), false)
	}

	key := args[0]

	storeMu.Lock()
	defer storeMu.Unlock()

	return lpush(key, args[1:])
}

func evalRPUSH(args []string) []byte {
	if len(args) < 2 {
		return Encode(errors.New("ERR wrong number of arguments for 'rpush' command"), false)
	}

	key := args[0]

	storeMu.Lock()
	defer storeMu.Unlock()

	return rpush(key, args[1:])
}

func evalLPOP(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'lpop' command"), false)
	}

	storeMu.Lock()
	defer storeMu.Unlock()

	return lpop(args[0])
}

func evalRPOP(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'rpop' command"), false)
	}

	storeMu.Lock()
	defer storeMu.Unlock()

	return rpop(args[0])
}

func evalLRANGE(args []string) []byte {
	if len(args) != 3 {
		return Encode(errors.New("ERR wrong number of arguments for 'lrange' command"), false)
	}

	storeMu.RLock()
	defer storeMu.RUnlock()

	return lrange(args[0], args[1], args[2])
}

// --- Internal variants (caller holds storeMu) ---

func evalLPUSHInternal(args []string) []byte {
	if len(args) < 2 {
		return Encode(errors.New("ERR wrong number of arguments for 'lpush' command"), false)
	}
	return lpush(args[0], args[1:])
}

func evalRPUSHInternal(args []string) []byte {
	if len(args) < 2 {
		return Encode(errors.New("ERR wrong number of arguments for 'rpush' command"), false)
	}
	return rpush(args[0], args[1:])
}

func evalLPOPInternal(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'lpop' command"), false)
	}
	return lpop(args[0])
}

func evalRPOPInternal(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'rpop' command"), false)
	}
	return rpop(args[0])
}

func evalLRANGEInternal(args []string) []byte {
	if len(args) != 3 {
		return Encode(errors.New("ERR wrong number of arguments for 'lrange' command"), false)
	}
	return lrange(args[0], args[1], args[2])
}

// --- Core list operations (caller must hold storeMu) ---

func lpush(key string, values []string) []byte {
	obj := get(key)

	if obj == nil {
		list := make([]string, len(values))
		for i, v := range values {
			list[len(values)-1-i] = v
		}
		put(key, NewObj(list, -1))
		return Encode(int64(len(list)), false)
	}

	list, ok := obj.Value.([]string)
	if !ok {
		return Encode(errors.New("WRONGTYPE Operation against a key holding the wrong kind of value"), false)
	}

	newList := make([]string, len(values)+len(list))
	for i, v := range values {
		newList[len(values)-1-i] = v
	}
	copy(newList[len(values):], list)
	obj.Value = newList
	keyVersions[key]++
	return Encode(int64(len(newList)), false)
}

func rpush(key string, values []string) []byte {
	obj := get(key)

	if obj == nil {
		list := make([]string, len(values))
		copy(list, values)
		put(key, NewObj(list, -1))
		return Encode(int64(len(list)), false)
	}

	list, ok := obj.Value.([]string)
	if !ok {
		return Encode(errors.New("WRONGTYPE Operation against a key holding the wrong kind of value"), false)
	}

	list = append(list, values...)
	obj.Value = list
	keyVersions[key]++
	return Encode(int64(len(list)), false)
}

func lpop(key string) []byte {
	obj := get(key)
	if obj == nil {
		return RESP_NIL
	}

	list, ok := obj.Value.([]string)
	if !ok {
		return Encode(errors.New("WRONGTYPE Operation against a key holding the wrong kind of value"), false)
	}

	if len(list) == 0 {
		return RESP_NIL
	}

	val := list[0]
	list = list[1:]

	if len(list) == 0 {
		del(key)
	} else {
		obj.Value = list
		keyVersions[key]++
	}

	return Encode(val, false)
}

func rpop(key string) []byte {
	obj := get(key)
	if obj == nil {
		return RESP_NIL
	}

	list, ok := obj.Value.([]string)
	if !ok {
		return Encode(errors.New("WRONGTYPE Operation against a key holding the wrong kind of value"), false)
	}

	if len(list) == 0 {
		return RESP_NIL
	}

	val := list[len(list)-1]
	list = list[:len(list)-1]

	if len(list) == 0 {
		del(key)
	} else {
		obj.Value = list
		keyVersions[key]++
	}

	return Encode(val, false)
}

func lrange(key, startStr, stopStr string) []byte {
	start, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil {
		return Encode(errors.New("ERR value is not an integer or out of range"), false)
	}
	stop, err := strconv.ParseInt(stopStr, 10, 64)
	if err != nil {
		return Encode(errors.New("ERR value is not an integer or out of range"), false)
	}

	obj := get(key)
	if obj == nil {
		return RESP_EMPTY_ARRAY
	}

	list, ok := obj.Value.([]string)
	if !ok {
		return Encode(errors.New("WRONGTYPE Operation against a key holding the wrong kind of value"), false)
	}

	length := int64(len(list))

	if start < 0 {
		start = length + start
	}
	if stop < 0 {
		stop = length + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= length {
		stop = length - 1
	}
	if start > stop || start >= length {
		return RESP_EMPTY_ARRAY
	}

	return EncodeBulkStringArray(list[start : stop+1])
}

func evalLLEN(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'llen' command"), false)
	}

	storeMu.RLock()
	defer storeMu.RUnlock()

	return llen(args[0])
}

func evalLLENInternal(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'llen' command"), false)
	}
	return llen(args[0])
}

func llen(key string) []byte {
	obj := get(key)
	if obj == nil {
		return RESP_ZERO
	}

	list, ok := obj.Value.([]string)
	if !ok {
		return Encode(errors.New("WRONGTYPE Operation against a key holding the wrong kind of value"), false)
	}

	return Encode(int64(len(list)), false)
}
