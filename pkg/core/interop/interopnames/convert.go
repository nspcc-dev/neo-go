package interopnames

import (
	"crypto/sha256"
	"encoding/binary"
)

// ToID returns an identificator of the method based on its name.
func ToID(name []byte) uint32 {
	h := sha256.Sum256(name)
	return binary.LittleEndian.Uint32(h[:4])
}
