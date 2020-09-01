package interopnames

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

var errNotFound = errors.New("interop not found")

// ToID returns an identificator of the method based on its name.
func ToID(name []byte) uint32 {
	h := sha256.Sum256(name)
	return binary.LittleEndian.Uint32(h[:4])
}

// FromID returns interop name from its id.
func FromID(id uint32) (string, error) {
	for i := range names {
		if id == ToID([]byte(names[i])) {
			return names[i], nil
		}
	}
	return "", errNotFound
}
