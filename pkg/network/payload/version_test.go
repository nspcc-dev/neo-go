package payload

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/network/capability"
	"github.com/stretchr/testify/assert"
)

func TestVersionEncodeDecode(t *testing.T) {
	var magic netmode.Magic = 56753
	var tcpPort uint16 = 3000
	var wsPort uint16 = 3001
	var id uint32 = 13337
	useragent := "/NEO:0.0.1/"
	var height uint32 = 100500
	var capabilities = []capability.Capability{
		{
			Type: capability.TCPServer,
			Data: &capability.Server{
				Port: tcpPort,
			},
		},
		{
			Type: capability.WSServer,
			Data: &capability.Server{
				Port: wsPort,
			},
		},
		{
			Type: capability.ArchivalNode,
			Data: &capability.Archival{},
		},
		{
			Type: 0xff,
			Data: &capability.Unknown{},
		},
		{
			Type: capability.FullNode,
			Data: &capability.Node{
				StartHeight: height,
			},
		},
		{
			Type: 0xf0,
			Data: &capability.Unknown{0x55, 0xaa},
		},
		{
			Type: capability.DisableCompressionNode,
			Data: &capability.DisableCompression{},
		},
	}

	version := NewVersion(magic, id, useragent, capabilities)
	versionDecoded := &Version{}
	testserdes.EncodeDecodeBinary(t, version, versionDecoded)

	assert.Equal(t, versionDecoded.Nonce, id)
	assert.ElementsMatch(t, capabilities, versionDecoded.Capabilities)
	assert.Equal(t, versionDecoded.UserAgent, []byte(useragent))
	assert.Equal(t, version, versionDecoded)
}
