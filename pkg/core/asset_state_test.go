package core

import (
	"bytes"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeAssetState(t *testing.T) {
	asset := &AssetState{
		ID:         util.RandomUint256(),
		AssetType:  transaction.Token,
		Name:       "super cool token",
		Amount:     util.Fixed8(1000000),
		Available:  util.Fixed8(100),
		Precision:  0,
		FeeMode:    feeMode,
		Owner:      &crypto.PublicKey{},
		Admin:      util.RandomUint160(),
		Issuer:     util.RandomUint160(),
		Expiration: 10,
		IsFrozen:   false,
	}

	buf := new(bytes.Buffer)
	assert.Nil(t, asset.EncodeBinary(buf))
	assetDecode := &AssetState{}
	assert.Nil(t, assetDecode.DecodeBinary(buf))
	assert.Equal(t, asset, assetDecode)
}
