package capability

// Type represents node capability type.
type Type byte

const (
	// TCPServer represents TCP node capability type.
	TCPServer Type = 0x01
	// WSServer represents WebSocket node capability type.
	WSServer Type = 0x02
	// DisableCompressionNode represents node capability that disables P2P
	// payloads compression.
	DisableCompressionNode Type = 0x03
	// FullNode represents a node that has complete current state.
	FullNode Type = 0x10
	// ArchivalNode represents a node that stores full block history.
	// These nodes can be used for P2P synchronization from genesis
	// (FullNode can cut the tail and may not respond to requests for
	// old (wrt MaxTraceableBlocks) blocks).
	ArchivalNode Type = 0x11

	// 0xf0-0xff are reserved for private experiments.
	ReservedFirst Type = 0xf0
	ReservedLast  Type = 0xff
)
