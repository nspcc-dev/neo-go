package result

type (
	// Version model used for reporting server version
	// info.
	Version struct {
		TCPPort   uint16 `json:"tcpport"`
		WSPort    uint16 `json:"wsport,omitempty"`
		Nonce     uint32 `json:"nonce"`
		UserAgent string `json:"useragent"`
	}
)
