//go:build !linux

package core

import "errors"

// FDComm is a stub on non-Linux platforms. The async (epoll) server
// is only available on Linux; keeping the type here lets the package
// compile everywhere while still failing loudly if it is ever used.
type FDComm struct {
	Fd int
}

func (f FDComm) Read(p []byte) (int, error) {
	return 0, errors.New("FDComm.Read: async server is only supported on linux")
}

func (f FDComm) Write(p []byte) (int, error) {
	return 0, errors.New("FDComm.Write: async server is only supported on linux")
}
