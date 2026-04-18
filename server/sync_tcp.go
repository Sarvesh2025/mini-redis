package server

import (
	"io"
	"log"
	"net"
	"strconv"

	"mini-redis/config"
)

func readCommand(c net.Conn) (string, error) {
	buf := make([]byte, 512)
	n, err := c.Read(buf)
	if err != nil {
		return "", err
	}
	return string(buf[:n]), nil
}

func respond(cmd string, c net.Conn) error {
	if _, err := c.Write([]byte(cmd)); err != nil {
		return err
	}
	return nil
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
			}

			log.Println("command", cmd)
			if err = respond(cmd, c); err != nil {
				log.Print("err write:", err)
			}
		}
	}
}
