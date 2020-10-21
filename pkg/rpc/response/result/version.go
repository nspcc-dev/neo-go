package result

import "github.com/nspcc-dev/neo-go/pkg/config/netmode"

type (
	// Version model used for reporting server version
	// info.
	Version struct {
		Magic     netmode.Magic `json:"magic"`
		TCPPort   uint16        `json:"tcpport"`
		WSPort    uint16        `json:"wsport,omitempty"`
		Nonce     uint32        `json:"nonce"`
		UserAgent string        `json:"useragent"`
	}
)
