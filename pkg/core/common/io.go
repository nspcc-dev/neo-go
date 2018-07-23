package util

// import (
// 	"encoding/binary"
// 	"io"
// )

// // Variable length integer, can be encoded to save space according to the value typed.
// // len 1 uint8
// // len 3 0xfd + uint16
// // len 5 0xfe = uint32
// // len 9 0xff = uint64
// // For more information about this:
// // https://github.com/neo-project/neo/wiki/Network-Protocol

// // ReadVarUint reads a variable unsigned integer and returns it as a uint64.
