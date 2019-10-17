package core

import (
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

const feeMode = 0x0

// Assets is mapping between AssetID and the AssetState.
type Assets map[util.Uint256]*AssetState

func (a Assets) commit(store storage.Store) error {
	for _, state := range a {
		if err := putAssetStateIntoStore(store, state); err != nil {
			return err
		}
	}
	return nil
}

// putAssetStateIntoStore puts given asset state into the given store.
func putAssetStateIntoStore(s storage.Store, as *AssetState) error {
	buf := io.NewBufBinWriter()
	as.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	key := storage.AppendPrefix(storage.STAsset, as.ID.Bytes())
	return s.Put(key, buf.Bytes())
}

// AssetState represents the state of an NEO registered Asset.
type AssetState struct {
	ID         util.Uint256
	AssetType  transaction.AssetType
	Name       string
	Amount     util.Fixed8
	Available  util.Fixed8
	Precision  uint8
	FeeMode    uint8
	FeeAddress util.Uint160
	Owner      *keys.PublicKey
	Admin      util.Uint160
	Issuer     util.Uint160
	Expiration uint32
	IsFrozen   bool
}

// DecodeBinary implements Serializable interface.
func (a *AssetState) DecodeBinary(br *io.BinReader) {
	br.ReadLE(&a.ID)
	br.ReadLE(&a.AssetType)

	a.Name = br.ReadString()

	br.ReadLE(&a.Amount)
	br.ReadLE(&a.Available)
	br.ReadLE(&a.Precision)
	br.ReadLE(&a.FeeMode)
	br.ReadLE(&a.FeeAddress)

	a.Owner = &keys.PublicKey{}
	a.Owner.DecodeBinary(br)
	br.ReadLE(&a.Admin)
	br.ReadLE(&a.Issuer)
	br.ReadLE(&a.Expiration)
	br.ReadLE(&a.IsFrozen)
}

// EncodeBinary implements Serializable interface.
func (a *AssetState) EncodeBinary(bw *io.BinWriter) {
	bw.WriteLE(a.ID)
	bw.WriteLE(a.AssetType)
	bw.WriteString(a.Name)
	bw.WriteLE(a.Amount)
	bw.WriteLE(a.Available)
	bw.WriteLE(a.Precision)
	bw.WriteLE(a.FeeMode)
	bw.WriteLE(a.FeeAddress)

	a.Owner.EncodeBinary(bw)

	bw.WriteLE(a.Admin)
	bw.WriteLE(a.Issuer)
	bw.WriteLE(a.Expiration)
	bw.WriteLE(a.IsFrozen)
}

// GetName returns the asset name based on its type.
func (a *AssetState) GetName() string {

	if a.AssetType == transaction.GoverningToken {
		return "NEO"
	} else if a.AssetType == transaction.UtilityToken {
		return "NEOGas"
	}

	return a.Name
}
