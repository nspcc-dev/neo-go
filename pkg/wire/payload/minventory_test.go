package payload

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/util"

	"github.com/stretchr/testify/assert"
)

func TestNewInventory(t *testing.T) {
	msgInv, err := NewInvMessage(InvTypeBlock)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	assert.Equal(t, command.Inv, msgInv.Command())

	hash, _ := util.Uint256DecodeBytes([]byte("hello"))
	if err = msgInv.AddHash(hash); err != nil {
		assert.Fail(t, "Error Adding a hash %s", err.Error())
	}

	for i := 0; i <= maxHashes+1; i++ {
		err = msgInv.AddHash(hash)
	}
	if err == nil {
		assert.Fail(t, "Max Hashes Exceeded, only allowed %v but have %v", maxHashes, len(msgInv.Hashes))
	}
}
func TestMaxHashes(t *testing.T) {
	msgInv, err := NewInvMessage(InvTypeBlock)
	if err != nil {
		assert.Fail(t, err.Error())
	}

	hash, _ := util.Uint256DecodeBytes([]byte("hello"))

	for i := 0; i <= maxHashes+1; i++ {
		err = msgInv.AddHash(hash)
	}
	if err == nil {
		assert.Fail(t, "Max Hashes Exceeded, only allowed %v but have %v", maxHashes, len(msgInv.Hashes))
	} else if err != MaxHashError {
		assert.Fail(t, "Expected a MaxHashError, however we got %s", err.Error())
	}
}
func TestEncodeDecodePayload(t *testing.T) {
	msgInv, err := NewInvMessage(InvTypeBlock)
	if err != nil {
		assert.Fail(t, err.Error())
	}

	blockOneHash := "d782db8a38b0eea0d7394e0f007c61c71798867578c77c387c08113903946cc9"
	hash, _ := util.Uint256DecodeString(blockOneHash)

	if err = msgInv.AddHash(hash); err != nil {
		assert.Fail(t, err.Error())
	}
	buf := new(bytes.Buffer)
	if err = msgInv.EncodePayload(buf); err != nil {
		assert.Fail(t, err.Error())
	}

	numOfHashes := []byte{0, 0, 0, 1}
	expected := append([]byte{byte(InvTypeBlock)}, numOfHashes...)
	expected = append(expected, hash.Bytes()...)

	assert.Equal(t, hex.EncodeToString(expected), hex.EncodeToString(buf.Bytes()))

	var InvDec InvMessage
	r := bytes.NewReader(buf.Bytes())
	if err = InvDec.DecodePayload(r); err != nil {
		assert.Fail(t, err.Error())
	}
	assert.Equal(t, 1, len(InvDec.Hashes))
	assert.Equal(t, blockOneHash, hex.EncodeToString(InvDec.Hashes[0].Bytes()))

}
func TestEmptyInv(t *testing.T) {
	msgInv, err := NewInvMessage(InvTypeBlock)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	buf := new(bytes.Buffer)
	msgInv.EncodePayload(buf)
	assert.Equal(t, []byte{byte(InvTypeBlock), 0, 0, 0, 0}, buf.Bytes())
	assert.Equal(t, 0, len(msgInv.Hashes))
}
