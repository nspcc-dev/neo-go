package capability

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/stretchr/testify/require"
)

func TestUnknownEncodeDecode(t *testing.T) {
	var (
		u  = Unknown{0x55, 0xaa}
		ud Unknown
	)
	testserdes.EncodeDecodeBinary(t, &u, &ud)
}

func TestArchivalEncodeDecode(t *testing.T) {
	var (
		a  = Archival{}
		ad Archival
	)
	testserdes.EncodeDecodeBinary(t, &a, &ad)

	var bad = []byte{0x02, 0x55, 0xaa} // Two-byte var-encoded string.
	require.Error(t, testserdes.DecodeBinary(bad, &ad))
}

func TestCheckUniqueError(t *testing.T) {
	// Successful cases are already checked in Version payload test.
	var caps Capabilities

	for _, bad := range [][]byte{
		{0x02, 0x11, 0x00, 0x11, 0x00},                                     // 2 Archival
		{0x02, 0x10, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x00}, // 2 FullNode
		{0x02, 0x01, 0x55, 0xaa, 0x01, 0x55, 0xaa},                         // 2 TCPServer
		{0x02, 0x02, 0x55, 0xaa, 0x02, 0x55, 0xaa},                         // 2 WSServer
	} {
		require.Error(t, testserdes.DecodeBinary(bad, &caps))
	}
	for _, good := range [][]byte{
		{0x02, 0x11, 0x00, 0x10, 0x00, 0x00, 0x00, 0x00}, // Archival + FullNode
		{0x02, 0x01, 0x55, 0xaa, 0x02, 0x55, 0xaa},       // TCPServer + WSServer
		{0x02, 0xf0, 0x00, 0xf0, 0x00},                   // 2 Reserved 0xf0
	} {
		require.NoError(t, testserdes.DecodeBinary(good, &caps))
	}
}
