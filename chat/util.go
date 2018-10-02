package chat

import (
	"fmt"
	"github.com/jpillora/backoff"
	"net"
	"os"
	"strconv"
	"time"
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

//
func BackoffDuration() func(error) {
	b := &backoff.Backoff{
		Min:    100 * time.Millisecond,
		Max:    60 * time.Second,
		Factor: 2,
		Jitter: false,
	}

	return func(rc error) {
		secs := b.Duration()
		fmt.Fprintf(os.Stdout, "rc: %v sleeping %v\n", rc, secs)
		time.Sleep(secs)
	}
}
