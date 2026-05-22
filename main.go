package main

import (
	"flag"
	"log"
	"os"
	"os/signal"

	"mini-redis/config"
	"mini-redis/core"
	"mini-redis/server"
)

func setupFlags() {
	flag.StringVar(&config.Host, "host", "0.0.0.0", "host for the mini-redis server")
	flag.IntVar(&config.Port, "port", 7379, "port for the mini-redis server")
	flag.IntVar(&config.KeysLimit, "keys-limit", 5*1000*1000, "maximum keys allowed before eviction kicks in")
	flag.BoolVar(&config.AOFEnabled, "aof-enabled", false, "enable AOF persistence")
	flag.StringVar(&config.AOFFile, "aof-file", "mini-redis.aof", "path to the AOF file")
	flag.Parse()
}

func main() {
	setupFlags()
	log.Println("starting mini-redis")

	if err := core.LoadAOF(); err != nil {
		log.Fatalf("AOF load failed: %v", err)
	}

	if err := core.InitAOF(); err != nil {
		log.Fatalf("AOF init failed: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		log.Println("shutting down, flushing AOF...")
		core.CloseAOF()
		os.Exit(0)
	}()

	if err := server.RunAsyncTCPServer(); err != nil {
		log.Println("async server unavailable:", err)
		log.Println("falling back to synchronous TCP server")
		server.RunSyncTCPServer()
	}
}
