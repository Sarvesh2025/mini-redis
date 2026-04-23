package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"

	"mini-redis/config"
	"mini-redis/core"
)

func readCommand(c net.Conn) (*core.RedisCmd, error) {
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

func respondError(err error, c net.Conn) {
	_, _ = c.Write([]byte(fmt.Sprintf("-%s\r\n", err)))
}

func respond(cmd *core.RedisCmd, c net.Conn) {
	err := core.EvalAndRespond(cmd, c)
	if err != nil {
		respondError(err, c)
	}
}

func RunSyncTCPServer() {
	log.Println("starting a synchronous TCP server on", config.Host, config.Port)

	var con_clients int = 0
	lsnr, err := net.Listen("tcp", config.Host+":"+strconv.Itoa(config.Port))
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	for {
		c, err := lsnr.Accept()
		if err != nil {
			log.Fatalf("accept: %v", err)
		}
		con_clients++
		log.Println("client connected with address:", c.RemoteAddr(), "concurrent clients", con_clients)

		for {
			cmd, err := readCommand(c)
			if err != nil {
				c.Close()
				con_clients--
				log.Println("client disconnected", c.RemoteAddr(), "concurrent clients", con_clients)
				if err == io.EOF {
					break
				}
				log.Println("err", err)
				break
			}

			respond(cmd, c)
		}
	}
}
