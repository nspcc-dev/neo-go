package core

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeAssetState(t *testing.T) {
	asset := &AssetState{
		ID:         randomUint256(),
		AssetType:  transaction.Token,
		Name:       "super cool token",
		Amount:     util.Fixed8(1000000),
		Available:  util.Fixed8(100),
		Precision:  0,
		FeeMode:    feeMode,
		Owner:      &keys.PublicKey{},
		Admin:      randomUint160(),
		Issuer:     randomUint160(),
		Expiration: 10,
		IsFrozen:   false,
	}

	buf := io.NewBufBinWriter()
	assert.Nil(t, asset.EncodeBinary(buf.BinWriter))
	assetDecode := &AssetState{}
	assert.Nil(t, assetDecode.DecodeBinary(io.NewBinReaderFromBuf(buf.Bytes())))
	assert.Equal(t, asset, assetDecode)
}
