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

// NewEndpoint creates an Endpoint from the given string.
func NewEndpoint(s string) (e Endpoint) {
	hostPort := strings.Split(s, ":")
	if len(hostPort) != 2 {
		return e
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

	return Endpoint{buf, uint16(p)}
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
