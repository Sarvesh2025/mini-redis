//go:build !linux

package core

import "errors"

type FDComm struct {
	Fd int
}

func (f FDComm) Read(p []byte) (int, error) {
	return 0, errors.New("FDComm.Read: async server is only supported on linux")
}

func (f FDComm) Write(p []byte) (int, error) {
	return 0, errors.New("FDComm.Write: async server is only supported on linux")
}
