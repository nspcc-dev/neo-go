package transaction

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestRegisterTX(t *testing.T) {
	someuint160, _ := util.Uint160DecodeString("4d3b96ae1bcc5a585e075e3b81920210dec16302")
	tx := &Transaction{
		Type:    RegisterType,
		Version: 0,
		Data: &RegisterTX{
			AssetType: UtilityToken,
			Name:      "this is some token I created",
			Amount:    util.Fixed8FromInt64(1000000),
			Precision: 8,
			Owner:     &keys.PublicKey{},
			Admin:     someuint160,
		},
	}

	buf := new(bytes.Buffer)
	assert.Nil(t, tx.EncodeBinary(buf))

	txDecode := &Transaction{}
	assert.Nil(t, txDecode.DecodeBinary(buf))
	txData := tx.Data.(*RegisterTX)
	txDecodeData := txDecode.Data.(*RegisterTX)
	assert.Equal(t, txData, txDecodeData)
	assert.Equal(t, tx.Hash(), txDecode.Hash())
}

func TestDecodeRegisterTXFromRawString(t *testing.T) {
	rawTX := "400000455b7b226c616e67223a227a682d434e222c226e616d65223a22e5b08fe89a81e882a1227d2c7b226c616e67223a22656e222c226e616d65223a22416e745368617265227d5d0000c16ff28623000000da1745e9b549bd0bfa1a569971c77eba30cd5a4b00000000"
	b, err := hex.DecodeString(rawTX)
	if err != nil {
		t.Fatal(err)
	}

	tx := &Transaction{}
	assert.Nil(t, tx.DecodeBinary(bytes.NewReader(b)))
	assert.Equal(t, RegisterType, tx.Type)
	txData := tx.Data.(*RegisterTX)
	assert.Equal(t, GoverningToken, txData.AssetType)
	assert.Equal(t, "[{\"lang\":\"zh-CN\",\"name\":\"小蚁股\"},{\"lang\":\"en\",\"name\":\"AntShare\"}]", txData.Name)
	assert.Equal(t, util.Fixed8FromInt64(100000000), txData.Amount)
	assert.Equal(t, uint8(0), txData.Precision)
	assert.Equal(t, &keys.PublicKey{}, txData.Owner)
	assert.Equal(t, "Abf2qMs1pzQb8kYk9RuxtUb9jtRKJVuBJt", crypto.AddressFromUint160(txData.Admin))
	assert.Equal(t, "c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b", tx.Hash().ReverseString())

	buf := new(bytes.Buffer)
	assert.Nil(t, tx.EncodeBinary(buf))

	txDecode := &Transaction{}
	assert.Nil(t, txDecode.DecodeBinary(buf))
	assert.Equal(t, tx, txDecode)
}
