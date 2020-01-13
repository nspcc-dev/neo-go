package result

type (
	// Version model used for reporting server version
	// info.
	Version struct {
		Port      uint16 `json:"port"`
		Nonce     uint32 `json:"nonce"`
		UserAgent string `json:"useragent"`
	}
)
