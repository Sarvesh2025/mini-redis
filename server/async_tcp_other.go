//go:build !linux

package server

import "errors"

// RunAsyncTCPServer is only implemented on Linux .
// On other platforms we return an error so main.go can fall back
// to the synchronous server.
func RunAsyncTCPServer() error {
	return errors.New("async TCP server (epoll) is only supported on linux; falling back to sync server")
}
