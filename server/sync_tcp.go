package server

import (
	"io"
	"log"
	"net"
	"strconv"

	"mini-redis/config"
	"mini-redis/core"
)

func RunSyncTCPServer() {
	log.Println("starting a synchronous TCP server on", config.Host, config.Port)

	var conClients int
	lsnr, err := net.Listen("tcp", config.Host+":"+strconv.Itoa(config.Port))
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	for {
		c, err := lsnr.Accept()
		if err != nil {
			log.Fatalf("accept: %v", err)
		}
		conClients++
		log.Println("client connected with address:", c.RemoteAddr(), "concurrent clients", conClients)

		ctx := core.NewClientContext()
		for {
			cmds, err := readCommands(c)
			if err != nil {
				c.Close()
				conClients--
				log.Println("client disconnected", c.RemoteAddr(), "concurrent clients", conClients)
				if err == io.EOF {
					break
				}
				log.Println("err", err)
				break
			}

			respond(cmds, c, ctx)
		}
	}
}
