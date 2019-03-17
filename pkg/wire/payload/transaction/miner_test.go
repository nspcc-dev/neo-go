package transaction

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeMiner(t *testing.T) {
	// transaction from mainnet a1f219dc6be4c35eca172e65e02d4591045220221b1543f1a4b67b9e9442c264

	rawtx := "0000fcd30e22000001e72d286979ee6cb1b7e65dfddfb2e384100b8d148e7758de42e4168b71792c60c8000000000000001f72e68b4e39602912106d53b229378a082784b200"
	rawtxBytes, _ := hex.DecodeString(rawtx)

	m := NewMiner(0)

	r := bytes.NewReader(rawtxBytes)
	err := m.Decode(r)
	assert.Equal(t, nil, err)

	assert.Equal(t, types.Miner, m.Type)
	assert.Equal(t, uint32(571397116), m.Nonce)

	assert.Equal(t, "a1f219dc6be4c35eca172e65e02d4591045220221b1543f1a4b67b9e9442c264", m.Hash.String())

	// Encode
	buf := new(bytes.Buffer)

	err = m.Encode(buf)
	assert.Equal(t, nil, err)

	assert.Equal(t, rawtxBytes, buf.Bytes())

}

func TestEncodeDecodeMiner2(t *testing.T) {
	// https://github.com/CityOfZion/neo-python/blob/master/neo/Core/TX/test_transactions.py#L109

	rawtx := "00006666654200000000"
	rawtxBytes, _ := hex.DecodeString(rawtx)

	m := NewMiner(0)

	r := bytes.NewReader(rawtxBytes)
	err := m.Decode(r)
	assert.Equal(t, nil, err)

	assert.Equal(t, int(m.Version), 0)

	//@todo: add the following assert once we have the Size calculation in place
	//assert.Equal(t, m.Size(), 10)

	assert.Equal(t, types.Miner, m.Type)
	assert.Equal(t, uint32(1113941606), m.Nonce)

	assert.Equal(t, "4c68669a54fa247d02545cff9d78352cb4a5059de7b3cd6ba82efad13953c9b9", m.Hash.String())

	// Encode
	buf := new(bytes.Buffer)

	err = m.Encode(buf)
	assert.Equal(t, nil, err)

	assert.Equal(t, rawtxBytes, buf.Bytes())

}
