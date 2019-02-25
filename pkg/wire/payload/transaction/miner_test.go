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
