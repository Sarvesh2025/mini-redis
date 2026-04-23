package main

import (
	"flag"
	"log"

	"mini-redis/config"
	"mini-redis/server"
)

func setupFlags() {
	flag.StringVar(&config.Host, "host", "0.0.0.0", "host for the mini-redis server")
	flag.IntVar(&config.Port, "port", 7379, "port for the mini-redis server")
	flag.Parse()
}

func main() {
	setupFlags()
	log.Println("starting mini-redis")

	if err := server.RunAsyncTCPServer(); err != nil {
		log.Println("async server unavailable:", err)
		log.Println("falling back to synchronous TCP server")
		server.RunSyncTCPServer()
	}
}
