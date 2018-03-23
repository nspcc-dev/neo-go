package crypto

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestComputeMerkleTree(t *testing.T) {
	rawHashes := []string{
		"fb5bd72b2d6792d75dc2f1084ffa9e9f70ca85543c717a6b13d9959b452a57d6",
		"c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b",
		"602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7",
		"3631f66024ca6f5b033d7e0809eb993443374830025af904fb51b0334f127cda",
	}

	hashes := make([]util.Uint256, len(rawHashes))
	for i, str := range rawHashes {
		hash, _ := util.Uint256DecodeString(str)
		hashes[i] = hash
	}

	merkle, err := NewMerkleTree(hashes)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "803ff4abe3ea6533bcc0be574efa02f83ae8fdc651c879056b0d9be336c01bf4", merkle.Root().String())
}
