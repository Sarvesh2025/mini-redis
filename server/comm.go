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

// readCommands reads a single read worth of bytes off the connection, decodes
// the (possibly pipelined) RESP arrays into a slice of RedisCmd. A single
// connection read may contain multiple commands when the client pipelines.
func readCommands(c io.ReadWriter) (core.RedisCmds, error) {
	// TODO: Max read in one shot is 512 bytes.
	// To allow input > 512 bytes, repeatedly read until delimiter/EOF.
	buf := make([]byte, 512)
	n, err := c.Read(buf)
	if err != nil {
		return nil, err
	}

	values, err := core.Decode(buf[:n])
	if err != nil {
		return nil, err
	}

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
