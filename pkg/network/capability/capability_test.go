package capability

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
)

func TestUnknownEncodeDecode(t *testing.T) {
	var (
		u  = Unknown{0x55, 0xaa}
		ud Unknown
	)
	testserdes.EncodeDecodeBinary(t, &u, &ud)
}
