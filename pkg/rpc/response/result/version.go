package result

type (
	// Version model used for reporting server version
	// info.
	Version struct {
		TCPPort   uint16 `json:"tcp_port"`
		WSPort    uint16 `json:"ws_port,omitempty"`
		Nonce     uint32 `json:"nonce"`
		UserAgent string `json:"user_agent"`
	}
)
