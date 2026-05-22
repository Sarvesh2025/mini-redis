package core

import (
	"bytes"
	"errors"
	"io"
	"log"
	"strconv"
	"strings"
	"time"
)

var (
	RESP_NIL     = []byte("$-1\r\n")
	RESP_OK      = []byte("+OK\r\n")
	RESP_ZERO    = []byte(":0\r\n")
	RESP_ONE     = []byte(":1\r\n")
	RESP_MINUS_1 = []byte(":-1\r\n")
	RESP_MINUS_2 = []byte(":-2\r\n")
)

func evalPING(args []string) []byte {
	if len(args) >= 2 {
		return Encode(errors.New("ERR wrong number of arguments for 'ping' command"), false)
	}

	if len(args) == 0 {
		return Encode("PONG", true)
	}
	return Encode(args[0], false)
}

func evalSET(args []string) []byte {
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

	Put(key, NewObj(value, exDurationMs))
	return RESP_OK
}

func evalGET(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'get' command"), false)
	}

	obj := Get(args[0])
	if obj == nil {
		return RESP_NIL
	}

	return Encode(obj.Value, false)
}

func evalTTL(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'ttl' command"), false)
	}

	switch v := TTL(args[0]); v {
	case -1:
		return RESP_MINUS_1
	case -2:
		return RESP_MINUS_2
	default:
		return Encode(v, false)
	}
}

func evalDEL(args []string) []byte {
	var countDeleted int64
	for _, key := range args {
		if ok := Del(key); ok {
			countDeleted++
		}
	}
	return Encode(countDeleted, false)
}

func evalEXPIRE(args []string) []byte {
	if len(args) != 2 {
		return Encode(errors.New("ERR wrong number of arguments for 'expire' command"), false)
	}

	key := args[0]
	exDurationSec, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return Encode(errors.New("ERR value is not an integer or out of range"), false)
	}

	obj := Get(key)
	if obj == nil {
		return RESP_ZERO
	}

	obj.ExpiresAt = time.Now().UnixMilli() + exDurationSec*1000
	return RESP_ONE
}

func EvalAndRespond(cmds RedisCmds, c io.ReadWriter) error {
	buf := bytes.NewBuffer(nil)

	for _, cmd := range cmds {
		var resp []byte
		switch cmd.Cmd {
		case "PING":
			resp = evalPING(cmd.Args)
		case "SET":
			resp = evalSET(cmd.Args)
			if bytes.Equal(resp, RESP_OK) {
				WriteAOF(cmd)
			}
		case "GET":
			resp = evalGET(cmd.Args)
		case "TTL":
			resp = evalTTL(cmd.Args)
		case "DEL":
			resp = evalDEL(cmd.Args)
			if !bytes.Equal(resp, RESP_ZERO) {
				WriteAOF(cmd)
			}
		case "EXPIRE":
			resp = evalEXPIRE(cmd.Args)
			if bytes.Equal(resp, RESP_ONE) {
				WriteAOF(cmd)
			}
		case "BGREWRITEAOF":
			go func() {
				if err := RewriteAOF(); err != nil {
					log.Println("AOF: rewrite error:", err)
				}
			}()
			resp = Encode("Background AOF rewrite started", true)
		default:
			resp = Encode(errors.New("ERR unknown command '"+strings.ToLower(cmd.Cmd)+"'"), false)
		}
		buf.Write(resp)
	}

	_, err := c.Write(buf.Bytes())
	return err
}
