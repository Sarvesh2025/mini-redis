//go:build linux

package core

import "syscall"

// FDComm wraps a raw file descriptor and implements io.ReadWriter
// so that the same readCommand / EvalAndRespond code paths used by the
// synchronous server can be reused by the epoll-based async server.
type FDComm struct {
	Fd int
}

func (f FDComm) Read(p []byte) (n int, err error) {
	return syscall.Read(f.Fd, p)
}

func (f FDComm) Write(p []byte) (n int, err error) {
	return syscall.Write(f.Fd, p)
}
