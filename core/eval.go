package core

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
)

func evalPING(args []string, c io.ReadWriter) error {
	var b []byte

	if len(args) >= 2 {
		return errors.New("ERR wrong number of arguments for 'ping' command")
	}

	if len(args) == 0 {
		b = Encode("PONG", true)
	} else {
		b = Encode(args[0], false)
	}

	_, err := c.Write(b)
	return err
}

func evalSET(args []string, c io.ReadWriter) error {
	if len(args) < 2 {
		return errors.New("ERR wrong number of arguments for 'set' command")
	}

	key := args[0]
	value := args[1]
	exDurationMs := int64(-1)

	for i := 2; i < len(args); i++ {
		switch strings.ToUpper(args[i]) {
		case "EX":
			i++
			if i == len(args) {
				return errors.New("ERR syntax error")
			}

			exDurationSec, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				return errors.New("ERR value is not an integer or out of range")
			}
			if exDurationSec <= 0 {
				return errors.New("ERR invalid expire time in 'set' command")
			}
			exDurationMs = exDurationSec * 1000
		default:
			return errors.New("ERR syntax error")
		}
	}

	Put(key, NewObj(value, exDurationMs))
	_, err := c.Write([]byte("+OK\r\n"))
	return err
}

func evalGET(args []string, c io.ReadWriter) error {
	if len(args) != 1 {
		return errors.New("ERR wrong number of arguments for 'get' command")
	}

	obj := Get(args[0])
	if obj == nil {
		_, err := c.Write(Encode(nil, false))
		return err
	}

	_, err := c.Write(Encode(obj.Value, false))
	return err
}

func evalTTL(args []string, c io.ReadWriter) error {
	if len(args) != 1 {
		return errors.New("ERR wrong number of arguments for 'ttl' command")
	}

	durationSec := TTL(args[0])
	_, err := c.Write(Encode(durationSec, false))
	return err
}

func evalDEL(args []string, c io.ReadWriter) error {
	var countDeleted int64
	for _, key := range args {
		if ok := Del(key); ok {
			countDeleted++
		}
	}
	_, err := c.Write(Encode(countDeleted, false))
	return err
}

func evalEXPIRE(args []string, c io.ReadWriter) error {
	if len(args) != 2 {
		return errors.New("ERR wrong number of arguments for 'expire' command")
	}

	key := args[0]
	exDurationSec, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return errors.New("ERR value is not an integer or out of range")
	}

	obj := Get(key)
	if obj == nil {
		_, err = c.Write([]byte(":0\r\n"))
		return err
	}

	obj.ExpiresAt = time.Now().UnixMilli() + exDurationSec*1000
	_, err = c.Write([]byte(":1\r\n"))
	return err
}

func EvalAndRespond(cmd *RedisCmd, c io.ReadWriter) error {
	switch cmd.Cmd {
	case "PING":
		return evalPING(cmd.Args, c)
	case "SET":
		return evalSET(cmd.Args, c)
	case "GET":
		return evalGET(cmd.Args, c)
	case "TTL":
		return evalTTL(cmd.Args, c)
	case "DEL":
		return evalDEL(cmd.Args, c)
	case "EXPIRE":
		return evalEXPIRE(cmd.Args, c)
	default:
		return errors.New("ERR unknown command '" + strings.ToLower(cmd.Cmd) + "'")
	}
}
