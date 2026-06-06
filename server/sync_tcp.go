package server

import (
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"mini-redis/config"
	"mini-redis/core"
)

func RunSyncTCPServer() {
	log.Println("starting a synchronous TCP server on", config.Host, config.Port)

	lsnr, err := net.Listen("tcp", config.Host+":"+strconv.Itoa(config.Port))
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	var conClients int64
	var wg sync.WaitGroup

	// Active expiry cron: run every 100ms like the async server
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			core.DeleteExpiredKeys()
		}
	}()

	for {
		c, err := lsnr.Accept()
		if err != nil {
			log.Println("accept error:", err)
			continue
		}

		atomic.AddInt64(&conClients, 1)
		log.Println("client connected with address:", c.RemoteAddr(), "concurrent clients", atomic.LoadInt64(&conClients))

		wg.Add(1)
		go func(conn net.Conn) {
			defer wg.Done()
			defer conn.Close()
			defer func() {
				n := atomic.AddInt64(&conClients, -1)
				log.Println("client disconnected", conn.RemoteAddr(), "concurrent clients", n)
			}()

			ctx := core.NewClientContext()
			for {
				cmds, err := readCommands(conn)
				if err != nil {
					if err != io.EOF {
						log.Println("read error:", err)
					}
					return
				}
				respond(cmds, conn, ctx)
			}
		}(c)
	}
}
