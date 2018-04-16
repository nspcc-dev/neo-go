package core

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/util"
)

const feeMode = 0x0

// Assets is mapping between AssetID and the AssetState.
type Assets map[util.Uint256]*AssetState

func (a Assets) commit(b storage.Batch) error {
	buf := new(bytes.Buffer)
	for hash, state := range a {
		if err := state.EncodeBinary(buf); err != nil {
			return err
		}
		key := storage.AppendPrefix(storage.STAsset, hash.Bytes())
		b.Put(key, buf.Bytes())
		buf.Reset()
	}
	return nil
}

// AssetState represents the state of an NEO registerd Asset.
type AssetState struct {
	ID         util.Uint256
	AssetType  transaction.AssetType
	Name       string
	Amount     util.Fixed8
	Available  util.Fixed8
	Precision  uint8
	FeeMode    uint8
	Owner      *crypto.PublicKey
	Admin      util.Uint160
	Issuer     util.Uint160
	Expiration uint32
	IsFrozen   bool
}

// DecodeBinary implements the Payload interface.
func (a *AssetState) DecodeBinary(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &a.ID); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &a.AssetType); err != nil {
		return err
	}

	var err error
	a.Name, err = util.ReadVarString(r)
	if err != nil {
		return err
	}

	if err := binary.Read(r, binary.LittleEndian, &a.Amount); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &a.Available); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &a.Precision); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &a.FeeMode); err != nil {
		return err
	}

	a.Owner = &crypto.PublicKey{}
	if err := a.Owner.DecodeBinary(r); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &a.Admin); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &a.Issuer); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &a.Expiration); err != nil {
		return err
	}
	return binary.Read(r, binary.LittleEndian, &a.IsFrozen)
}

// EncodeBinary implements the Payload interface.
func (a *AssetState) EncodeBinary(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, a.ID); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, a.AssetType); err != nil {
		return err
	}
	if err := util.WriteVarUint(w, uint64(len(a.Name))); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, []byte(a.Name)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, a.Amount); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, a.Available); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, a.Precision); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, a.FeeMode); err != nil {
		return err
	}
	if err := a.Owner.EncodeBinary(w); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, a.Admin); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, a.Issuer); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, a.Expiration); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, a.IsFrozen)
}
