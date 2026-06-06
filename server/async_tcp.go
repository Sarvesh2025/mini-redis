//go:build linux

package server

import (
	"errors"
	"log"
	"net"
	"syscall"
	"time"

	"mini-redis/config"
	"mini-redis/core"
)

var conClients int

func RunAsyncTCPServer() error {
	log.Println("starting an asynchronous TCP server on", config.Host, config.Port)

	maxClients := 20000
	events := make([]syscall.EpollEvent, maxClients)
	clientContexts := make(map[int]*core.ClientContext)

	serverFD, err := syscall.Socket(syscall.AF_INET, syscall.O_NONBLOCK|syscall.SOCK_STREAM, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(serverFD)

	if err = syscall.SetNonblock(serverFD, true); err != nil {
		return err
	}
	ip4 := net.ParseIP(config.Host).To4()
	if ip4 == nil {
		return errors.New("invalid IPv4 address: " + config.Host)
	}
	if err = syscall.Bind(serverFD, &syscall.SockaddrInet4{
		Port: config.Port,
		Addr: [4]byte{ip4[0], ip4[1], ip4[2], ip4[3]},
	}); err != nil {
		return err
	}

	if err = syscall.Listen(serverFD, maxClients); err != nil {
		return err
	}

	epollFD, err := syscall.EpollCreate1(0)
	if err != nil {
		log.Fatal(err)
	}
	defer syscall.Close(epollFD)
	socketServerEvent := syscall.EpollEvent{
		Events: syscall.EPOLLIN,
		Fd:     int32(serverFD),
	}
	if err = syscall.EpollCtl(epollFD, syscall.EPOLL_CTL_ADD, serverFD, &socketServerEvent); err != nil {
		return err
	}

	lastCronExecTime := time.Now()
	cronFrequency := 100 * time.Millisecond

	for {
		if time.Now().After(lastCronExecTime.Add(cronFrequency)) {
			core.DeleteExpiredKeys()
			lastCronExecTime = time.Now()
		}

		nevents, e := syscall.EpollWait(epollFD, events[:], int(cronFrequency.Milliseconds()))
		if e != nil {
			continue
		}

		for i := 0; i < nevents; i++ {
			if int(events[i].Fd) == serverFD {
				fd, _, err := syscall.Accept(serverFD)
				if err != nil {
					log.Println("err", err)
					continue
				}

				conClients++
				_ = syscall.SetNonblock(fd, true)
				clientContexts[fd] = core.NewClientContext()

				socketClientEvent := syscall.EpollEvent{
					Events: syscall.EPOLLIN,
					Fd:     int32(fd),
				}
				if err := syscall.EpollCtl(epollFD, syscall.EPOLL_CTL_ADD, fd, &socketClientEvent); err != nil {
					log.Fatal(err)
				}
			} else {
				fd := int(events[i].Fd)
				comm := core.FDComm{Fd: fd}
				cmds, err := readCommands(comm)
				if err != nil {
					syscall.Close(fd)
					delete(clientContexts, fd)
					conClients--
					log.Println("client disconnected. concurrent clients:", conClients)
					continue
				}
				ctx := clientContexts[fd]
				respond(cmds, comm, ctx)
			}
		}
	}
}
