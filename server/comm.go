package server

import (
	"fmt"
	"io"
	"strings"

	"mini-redis/core"
)

func readCommand(c io.ReadWriter) (*core.RedisCmd, error) {
	// TODO: Max read in one shot is 512 bytes.
	// To allow input > 512 bytes, repeatedly read until delimiter/EOF.
	buf := make([]byte, 512)
	n, err := c.Read(buf)
	if err != nil {
		return nil, err
	}

	tokens, err := core.DecodeArrayString(buf[:n])
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	return &core.RedisCmd{
		Cmd:  strings.ToUpper(tokens[0]),
		Args: tokens[1:],
	}, nil
}

func respondError(err error, c io.Writer) {
	_, _ = c.Write([]byte(fmt.Sprintf("-%s\r\n", err)))
}

func respond(cmd *core.RedisCmd, c io.ReadWriter) {
	if err := core.EvalAndRespond(cmd, c); err != nil {
		respondError(err, c)
	}
}
