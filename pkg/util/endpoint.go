package util

import (
	"fmt"
	"strconv"
	"strings"
)

// Endpoint host + port of a node, compatible with net.Addr.
type Endpoint struct {
	IP   [16]byte // TODO: make a uint128 type
	Port uint16
}

// EndpointFromString returns an Endpoint from the given string.
// For now this only handles the most simple hostport form.
// e.g. 127.0.0.1:3000
// This should be enough to work with for now.
func EndpointFromString(s string) (Endpoint, error) {
	hostPort := strings.Split(s, ":")
	if len(hostPort) != 2 {
		return Endpoint{}, fmt.Errorf("invalid address string: %s", s)
	}
	host := hostPort[0]
	port := hostPort[1]

	ch := strings.Split(host, ".")

	buf := [16]byte{}
	var n int
	for i := 0; i < len(ch); i++ {
		n = 12 + i
		nn, _ := strconv.Atoi(ch[i])
		buf[n] = byte(nn)
	}

	p, _ := strconv.Atoi(port)

	return Endpoint{buf, uint16(p)}, nil
}

// Network implements the net.Addr interface.
func (e Endpoint) Network() string { return "tcp" }

// String implements the net.Addr interface.
func (e Endpoint) String() string {
	b := make([]uint8, 4)
	for i := 0; i < 4; i++ {
		b[i] = byte(e.IP[len(e.IP)-4+i])
	}
	return fmt.Sprintf("%d.%d.%d.%d:%d", b[0], b[1], b[2], b[3], e.Port)
}
