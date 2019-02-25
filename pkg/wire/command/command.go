package command

// Size of command field in bytes
const (
	Size = 12
)

// CommandType represents the type of a message command.
type Type string

// Valid protocol commands used to send between nodes.
// use this to get
const (
	Version    Type = "version"
	Mempool    Type = "mempool"
	Ping       Type = "ping"
	Pong       Type = "pong"
	Verack     Type = "verack"
	GetAddr    Type = "getaddr"
	Addr       Type = "addr"
	GetHeaders Type = "getheaders"
	Headers    Type = "headers"
	GetBlocks  Type = "getblocks"
	Inv        Type = "inv"
	GetData    Type = "getdata"
	Block      Type = "block"
	TX         Type = "tx"
	Consensus  Type = "consensus"
	Unknown    Type = "unknown"
)
