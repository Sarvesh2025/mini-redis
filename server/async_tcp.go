//go:build linux

package server

import (
	"log"
	"net"
	"syscall"

	"mini-redis/config"
	"mini-redis/core"
)

var conClients int

// RunAsyncTCPServer runs an epoll-based, single-threaded, non-blocking
// TCP server capable of handling many thousands of concurrent clients
// without spawning a goroutine per connection.
func RunAsyncTCPServer() error {
	log.Println("starting an asynchronous TCP server on", config.Host, config.Port)

	maxClients := 20000

	// EPOLL event buffer used on every epoll_wait call.
	events := make([]syscall.EpollEvent, maxClients)

	// Create a non-blocking listening socket.
	serverFD, err := syscall.Socket(syscall.AF_INET, syscall.O_NONBLOCK|syscall.SOCK_STREAM, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(serverFD)

	if err = syscall.SetNonblock(serverFD, true); err != nil {
		return err
	}

	// Bind to host:port.
	ip4 := net.ParseIP(config.Host)
	if err = syscall.Bind(serverFD, &syscall.SockaddrInet4{
		Port: config.Port,
		Addr: [4]byte{ip4[0], ip4[1], ip4[2], ip4[3]},
	}); err != nil {
		return err
	}

	if err = syscall.Listen(serverFD, maxClients); err != nil {
		return err
	}

	// ---- AsyncIO starts here ----

	epollFD, err := syscall.EpollCreate1(0)
	if err != nil {
		log.Fatal(err)
	}
	defer syscall.Close(epollFD)

	// Register the listening socket with epoll for read readiness.
	socketServerEvent := syscall.EpollEvent{
		Events: syscall.EPOLLIN,
		Fd:     int32(serverFD),
	}
	if err = syscall.EpollCtl(epollFD, syscall.EPOLL_CTL_ADD, serverFD, &socketServerEvent); err != nil {
		return err
	}

	for {
		// Wait for any FD to become ready for I/O.
		nevents, e := syscall.EpollWait(epollFD, events[:], -1)
		if e != nil {
			continue
		}

		for i := 0; i < nevents; i++ {
			if int(events[i].Fd) == serverFD {
				// Listener is ready -> accept a new client.
				fd, _, err := syscall.Accept(serverFD)
				if err != nil {
					log.Println("err", err)
					continue
				}

				conClients++
				_ = syscall.SetNonblock(fd, true)

				// Start monitoring this new client FD for read readiness.
				socketClientEvent := syscall.EpollEvent{
					Events: syscall.EPOLLIN,
					Fd:     int32(fd),
				}
				if err := syscall.EpollCtl(epollFD, syscall.EPOLL_CTL_ADD, fd, &socketClientEvent); err != nil {
					log.Fatal(err)
				}
			} else {
				// Existing client has data to read.
				comm := core.FDComm{Fd: int(events[i].Fd)}
				cmd, err := readCommand(comm)
				if err != nil {
					syscall.Close(int(events[i].Fd))
					conClients--
					log.Println("client disconnected. concurrent clients:", conClients)
					continue
				}
				respond(cmd, comm)
			}
		}
	}
}
