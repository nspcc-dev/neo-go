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

	assert.Equal(t, nil, err)
	assert.Equal(t, command.Inv, msgInv.Command())

	hash, _ := util.Uint256DecodeBytes([]byte("hello"))
	err = msgInv.AddHash(hash)
	assert.Equal(t, nil, err)
}

// Adjust test time or it will timeout
// func TestMaxHashes(t *testing.T) {
// 	msgInv, err := NewInvMessage(InvTypeBlock)
// 	assert.Equal(t, nil, err)

// 	hash, _ := util.Uint256DecodeBytes([]byte("hello"))

// 	for i := 0; i <= maxHashes+1; i++ {
// 		err = msgInv.AddHash(hash)
// 	}
// 	if err == nil {
// 		assert.Fail(t, "Max Hashes Exceeded, only allowed %v but have %v", maxHashes, len(msgInv.Hashes))
// 	} else if err != MaxHashError {
// 		assert.Fail(t, "Expected a MaxHashError, however we got %s", err.Error())
// 	}
// }
func TestEncodeDecodePayload(t *testing.T) {
	msgInv, err := NewInvMessage(InvTypeBlock)
	assert.Equal(t, nil, err)

	blockOneHash := "d782db8a38b0eea0d7394e0f007c61c71798867578c77c387c08113903946cc9"
	hash, _ := util.Uint256DecodeString(blockOneHash)

	err = msgInv.AddHash(hash)
	assert.Equal(t, nil, err)

	buf := new(bytes.Buffer)
	err = msgInv.EncodePayload(buf)
	assert.Equal(t, nil, err)

	numOfHashes := []byte{1}
	expected := append([]byte{uint8(InvTypeBlock)}, numOfHashes...)
	expected = append(expected, hash.Bytes()...)

	assert.Equal(t, hex.EncodeToString(expected), hex.EncodeToString(buf.Bytes()))

	var InvDec InvMessage
	r := bytes.NewReader(buf.Bytes())
	err = InvDec.DecodePayload(r)
	assert.Equal(t, nil, err)

	assert.Equal(t, 1, len(InvDec.Hashes))
	assert.Equal(t, blockOneHash, hex.EncodeToString(InvDec.Hashes[0].Bytes()))

}
func TestEmptyInv(t *testing.T) {
	msgInv, err := NewInvMessage(InvTypeBlock)
	assert.Equal(t, nil, err)

	buf := new(bytes.Buffer)
	msgInv.EncodePayload(buf)
	assert.Equal(t, []byte{byte(InvTypeBlock), 0}, buf.Bytes())
	assert.Equal(t, 0, len(msgInv.Hashes))
}
