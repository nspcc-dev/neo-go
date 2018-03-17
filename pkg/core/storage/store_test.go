package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	prefixes = []KeyPrefix{
		DataBlock,
		DataTransaction,
		STAccount,
		STCoin,
		STValidator,
		STAsset,
		STContract,
		STStorage,
		IXHeaderHashList,
		IXValidatorsCount,
		SYSCurrentBlock,
		SYSCurrentHeader,
		SYSVersion,
	}

	expected = []uint8{
		0x01,
		0x02,
		0x40,
		0x44,
		0x48,
		0x4c,
		0x50,
		0x70,
		0x80,
		0x90,
		0xc0,
		0xc1,
		0xf0,
	}
)

func TestAppendPrefix(t *testing.T) {
	for i := 0; i < len(expected); i++ {
		value := []byte{0x01, 0x02}
		prefix := AppendPrefix(prefixes[i], value)
		assert.Equal(t, KeyPrefix(expected[i]), KeyPrefix(prefix[0]))
	}
}

func TestAppendPrefixInt(t *testing.T) {
	for i := 0; i < len(expected); i++ {
		value := 2000
		prefix := AppendPrefixInt(prefixes[i], value)
		assert.Equal(t, KeyPrefix(expected[i]), KeyPrefix(prefix[0]))
	}
}
