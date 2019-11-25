package entities

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/testutil"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeAssetState(t *testing.T) {
	asset := &AssetState{
		ID:         testutil.RandomUint256(),
		AssetType:  transaction.Token,
		Name:       "super cool token",
		Amount:     util.Fixed8(1000000),
		Available:  util.Fixed8(100),
		Precision:  0,
		FeeMode:    feeMode,
		Owner:      keys.PublicKey{},
		Admin:      testutil.RandomUint160(),
		Issuer:     testutil.RandomUint160(),
		Expiration: 10,
		IsFrozen:   false,
	}

	buf := io.NewBufBinWriter()
	asset.EncodeBinary(buf.BinWriter)
	assert.Nil(t, buf.Err)
	assetDecode := &AssetState{}
	r := io.NewBinReaderFromBuf(buf.Bytes())
	assetDecode.DecodeBinary(r)
	assert.Nil(t, r.Err)
	assert.Equal(t, asset, assetDecode)
}
