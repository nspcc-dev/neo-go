package capability

// Type represents node capability type.
type Type byte

const (
	// TCPServer represents TCP node capability type.
	TCPServer Type = 0x01
	// WSServer represents WebSocket node capability type.
	WSServer Type = 0x02
	// FullNode represents full node capability type.
	FullNode Type = 0x10
)
