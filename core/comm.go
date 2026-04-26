//go:build linux

package core

import "syscall"

type FDComm struct {
	Fd int
}

func (f FDComm) Read(p []byte) (n int, err error) {
	return syscall.Read(f.Fd, p)
}

func (f FDComm) Write(p []byte) (n int, err error) {
	return syscall.Write(f.Fd, p)
}
