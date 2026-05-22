package core

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	RESP_QUEUED    = []byte("+QUEUED\r\n")
	RESP_NIL_ARRAY = []byte("*-1\r\n")
)

func evalMULTI(ctx *ClientContext) []byte {
	if ctx.InMulti {
		return Encode(errors.New("ERR MULTI calls can not be nested"), false)
	}
	ctx.InMulti = true
	ctx.TxQueue = nil
	return RESP_OK
}

func evalEXEC(ctx *ClientContext) []byte {
	if !ctx.InMulti {
		return Encode(errors.New("ERR EXEC without MULTI"), false)
	}

	storeMu.Lock()

	if ctx.WatchedKeys != nil {
		for k, ver := range ctx.WatchedKeys {
			if keyVersions[k] != ver {
				storeMu.Unlock()
				ctx.Reset()
				return RESP_NIL_ARRAY
			}
		}
	}

	responses := make([][]byte, len(ctx.TxQueue))
	for i, cmd := range ctx.TxQueue {
		responses[i] = evalCmdInternal(cmd)
	}

	storeMu.Unlock()

	for i, cmd := range ctx.TxQueue {
		switch cmd.Cmd {
		case "SET":
			if bytes.Equal(responses[i], RESP_OK) {
				WriteAOF(cmd)
			}
		case "DEL":
			if !bytes.Equal(responses[i], RESP_ZERO) {
				WriteAOF(cmd)
			}
		case "EXPIRE":
			if bytes.Equal(responses[i], RESP_ONE) {
				WriteAOF(cmd)
			}
		}
	}

	ctx.Reset()
	return encodeArrayOfRaw(responses)
}

func evalDISCARD(ctx *ClientContext) []byte {
	if !ctx.InMulti {
		return Encode(errors.New("ERR DISCARD without MULTI"), false)
	}
	ctx.Reset()
	return RESP_OK
}

func evalWATCH(args []string, ctx *ClientContext) []byte {
	if ctx.InMulti {
		return Encode(errors.New("ERR WATCH inside MULTI is not allowed"), false)
	}
	if len(args) == 0 {
		return Encode(errors.New("ERR wrong number of arguments for 'watch' command"), false)
	}
	if ctx.WatchedKeys == nil {
		ctx.WatchedKeys = make(map[string]uint64)
	}
	storeMu.RLock()
	for _, key := range args {
		ctx.WatchedKeys[key] = keyVersions[key]
	}
	storeMu.RUnlock()
	return RESP_OK
}

// --- Internal eval functions (called during EXEC under storeMu.Lock) ---

func evalCmdInternal(cmd *RedisCmd) []byte {
	switch cmd.Cmd {
	case "PING":
		return evalPING(cmd.Args)
	case "SET":
		return evalSETInternal(cmd.Args)
	case "GET":
		return evalGETInternal(cmd.Args)
	case "TTL":
		return evalTTLInternal(cmd.Args)
	case "DEL":
		return evalDELInternal(cmd.Args)
	case "EXPIRE":
		return evalEXPIREInternal(cmd.Args)
	default:
		return Encode(errors.New("ERR unknown command '"+strings.ToLower(cmd.Cmd)+"'"), false)
	}
}

func evalSETInternal(args []string) []byte {
	if len(args) <= 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'set' command"), false)
	}

	key, value := args[0], args[1]
	exDurationMs := int64(-1)

	for i := 2; i < len(args); i++ {
		switch strings.ToUpper(args[i]) {
		case "EX":
			i++
			if i == len(args) {
				return Encode(errors.New("ERR syntax error"), false)
			}
			exDurationSec, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				return Encode(errors.New("ERR value is not an integer or out of range"), false)
			}
			if exDurationSec <= 0 {
				return Encode(errors.New("ERR invalid expire time in 'set' command"), false)
			}
			exDurationMs = exDurationSec * 1000
		default:
			return Encode(errors.New("ERR syntax error"), false)
		}
	}

	put(key, NewObj(value, exDurationMs))
	return RESP_OK
}

func evalGETInternal(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'get' command"), false)
	}
	obj := get(args[0])
	if obj == nil {
		return RESP_NIL
	}
	return Encode(obj.Value, false)
}

func evalTTLInternal(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'ttl' command"), false)
	}
	switch v := getTTL(args[0]); v {
	case -1:
		return RESP_MINUS_1
	case -2:
		return RESP_MINUS_2
	default:
		return Encode(v, false)
	}
}

func evalDELInternal(args []string) []byte {
	var countDeleted int64
	for _, key := range args {
		if ok := del(key); ok {
			countDeleted++
		}
	}
	return Encode(countDeleted, false)
}

func evalEXPIREInternal(args []string) []byte {
	if len(args) != 2 {
		return Encode(errors.New("ERR wrong number of arguments for 'expire' command"), false)
	}
	key := args[0]
	exDurationSec, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return Encode(errors.New("ERR value is not an integer or out of range"), false)
	}
	if !setExpire(key, time.Now().UnixMilli()+exDurationSec*1000) {
		return RESP_ZERO
	}
	return RESP_ONE
}

func encodeArrayOfRaw(responses [][]byte) []byte {
	buf := bytes.NewBuffer(nil)
	buf.WriteString(fmt.Sprintf("*%d\r\n", len(responses)))
	for _, r := range responses {
		buf.Write(r)
	}
	return buf.Bytes()
}
