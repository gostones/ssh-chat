package chat

import (
	"net"
	"strconv"
)

// ParseInt parses string into int or returns the value in the second arg
func ParseInt(s string, v int) int {
	if s == "" {
		return v
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		i = v
	}
	return i
}

// FreePort returns a free port
func FreePort() int {
	l, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		return -1
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}
