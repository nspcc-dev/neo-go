package result

type (
	// Version model used for reporting server version
	// info.
	Version struct {
		TCPPort   uint16 `json:"tcpPort"`
		WSPort    uint16 `json:"wsPort,omitempty"`
		Nonce     uint32 `json:"nonce"`
		UserAgent string `json:"userAgent"`
	}
)
