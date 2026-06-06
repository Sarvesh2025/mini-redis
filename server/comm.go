package server

import (
	"errors"
	"io"
	"strings"

	"mini-redis/core"
)

func toArrayString(ai []interface{}) ([]string, error) {
	as := make([]string, len(ai))
	for i := range ai {
		s, ok := ai[i].(string)
		if !ok {
			return nil, errors.New("expected array of strings")
		}
		as[i] = s
	}
	return as, nil
}

// readCommands reads bytes off the connection until a complete set of RESP
// commands is decoded. Handles both blocking (sync server) and non-blocking
// (async server with epoll) sockets.
func readCommands(c io.ReadWriter) (core.RedisCmds, error) {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)

	for {
		n, err := c.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}

		// n==0 with no error on a non-blocking socket means peer closed
		if n == 0 && err == nil {
			return nil, io.EOF
		}

		if err != nil {
			if len(buf) == 0 {
				return nil, err
			}
			break
		}

		values, decErr := core.Decode(buf)
		if decErr == nil || len(values) > 0 {
			return buildCmds(values)
		}

		if len(buf) > 65536 {
			return nil, errors.New("request too large")
		}
	}

	values, _ := core.Decode(buf)
	if len(values) == 0 {
		return nil, errors.New("incomplete RESP data")
	}
	return buildCmds(values)
}

func buildCmds(values []interface{}) (core.RedisCmds, error) {
	cmds := make(core.RedisCmds, 0, len(values))
	for _, value := range values {
		array, ok := value.([]interface{})
		if !ok {
			return nil, errors.New("expected RESP array per command")
		}
		tokens, err := toArrayString(array)
		if err != nil {
			return nil, err
		}
		if len(tokens) == 0 {
			continue
		}
		cmds = append(cmds, &core.RedisCmd{
			Cmd:  strings.ToUpper(tokens[0]),
			Args: tokens[1:],
		})
	}
	return cmds, nil
}

func respond(cmds core.RedisCmds, c io.ReadWriter, ctx *core.ClientContext) {
	if err := core.EvalAndRespond(cmds, c, ctx); err != nil {
		_, _ = c.Write([]byte("-" + err.Error() + "\r\n"))
	}
}
