package core

import (
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// Validators is a mapping between public keys and ValidatorState.
type Validators map[*keys.PublicKey]*ValidatorState

func (v Validators) getAndUpdate(s storage.Store, publicKey *keys.PublicKey) (*ValidatorState, error) {
	if validator, ok := v[publicKey]; ok {
		return validator, nil
	}

	validatorState, err := getValidatorStateFromStore(s, publicKey)
	if err != nil {
		if err != storage.ErrKeyNotFound {
			return nil, err
		}
		validatorState = &ValidatorState{PublicKey: publicKey}
	}
	v[publicKey] = validatorState
	return validatorState, nil

}

// getValidatorsFromStore returns all validators from store.
func getValidatorsFromStore(s storage.Store) []*ValidatorState {
	var validators []*ValidatorState
	s.Seek(storage.STValidator.Bytes(), func(k, v []byte) {
		r := io.NewBinReaderFromBuf(v)
		validator := &ValidatorState{}
		validator.DecodeBinary(r)
		if r.Err != nil {
			return
		}
		validators = append(validators, validator)
	})
	return validators
}

// getValidatorStateFromStore returns validator by publicKey.
func getValidatorStateFromStore(s storage.Store, publicKey *keys.PublicKey) (*ValidatorState, error) {
	validatorState := &ValidatorState{}
	key := storage.AppendPrefix(storage.STValidator, publicKey.Bytes())
	if b, err := s.Get(key); err == nil {
		r := io.NewBinReaderFromBuf(b)
		validatorState.DecodeBinary(r)
		if r.Err != nil {
			return nil, fmt.Errorf("failed to decode (ValidatorState): %s", r.Err)
		}
	} else {
		return nil, err
	}
	return validatorState, nil
}

// commit writes all validator states to the given Batch.
func (v Validators) commit(store storage.Store) error {
	for _, validator := range v {
		if err := putValidatorStateIntoStore(store, validator); err != nil {
			return err
		}
	}
	return nil
}

// putValidatorStateIntoStore puts given ValidatorState into the given store.
func putValidatorStateIntoStore(store storage.Store, vs *ValidatorState) error {
	buf := io.NewBufBinWriter()
	vs.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	key := storage.AppendPrefix(storage.STValidator, vs.PublicKey.Bytes())
	return store.Put(key, buf.Bytes())
}

// ValidatorState holds the state of a validator.
type ValidatorState struct {
	PublicKey  *keys.PublicKey
	Registered bool
	Votes      util.Fixed8
}

// RegisteredAndHasVotes returns true or false whether Validator is registered and has votes.
func (vs *ValidatorState) RegisteredAndHasVotes() bool {
	return vs.Registered && vs.Votes > util.Fixed8(0)
}

// EncodeBinary encodes ValidatorState to the given BinWriter.
func (vs *ValidatorState) EncodeBinary(bw *io.BinWriter) {
	vs.PublicKey.EncodeBinary(bw)
	bw.WriteLE(vs.Registered)
	bw.WriteLE(vs.Votes)
}

// DecodeBinary decodes ValidatorState from the given BinReader.
func (vs *ValidatorState) DecodeBinary(reader *io.BinReader) {
	vs.PublicKey = &keys.PublicKey{}
	vs.PublicKey.DecodeBinary(reader)
	reader.ReadLE(&vs.Registered)
	reader.ReadLE(&vs.Votes)
}
