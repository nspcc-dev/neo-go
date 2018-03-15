package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	prefixes = []keyPrefix{
		preDataBlock,
		preDataTransaction,
		preSTAccount,
		preSTCoin,
		preSTValidator,
		preSTAsset,
		preSTContract,
		preSTStorage,
		preIXHeaderHashList,
		preIXValidatorsCount,
		preSYSCurrentBlock,
		preSYSCurrentHeader,
		preSYSVersion,
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
		prefix := appendPrefix(prefixes[i], value)
		assert.Equal(t, keyPrefix(expected[i]), keyPrefix(prefix[0]))
	}
}

func TestAppendPrefixInt(t *testing.T) {
	for i := 0; i < len(expected); i++ {
		value := 2000
		prefix := appendPrefixInt(prefixes[i], value)
		assert.Equal(t, keyPrefix(expected[i]), keyPrefix(prefix[0]))
	}
}
