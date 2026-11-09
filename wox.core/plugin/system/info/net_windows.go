//go:build windows

package info

import "net"

const (
	netFlagUp       = net.FlagUp
	netFlagLoopback = net.FlagLoopback
)

func netInterfaces() ([]net.Interface, error) {
	return net.Interfaces()
}
