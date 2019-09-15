package core

import (
	"bytes"
	"fmt"
	"io"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// Accounts is mapping between a account address and AccountState.
type Accounts map[util.Uint160]*AccountState

func (a Accounts) getAndUpdate(s storage.Store, hash util.Uint160) (*AccountState, error) {
	if account, ok := a[hash]; ok {
		return account, nil
	}

	account := &AccountState{}
	key := storage.AppendPrefix(storage.STAccount, hash.Bytes())
	if b, err := s.Get(key); err == nil {
		if err := account.DecodeBinary(bytes.NewReader(b)); err != nil {
			return nil, fmt.Errorf("failed to decode (AccountState): %s", err)
		}
	} else {
		account = NewAccountState(hash)
	}

	a[hash] = account
	return account, nil
}

// commit writes all account states to the given Batch.
func (a Accounts) commit(b storage.Batch) error {
	buf := new(bytes.Buffer)
	for hash, state := range a {
		if err := state.EncodeBinary(buf); err != nil {
			return err
		}
		key := storage.AppendPrefix(storage.STAccount, hash.Bytes())
		b.Put(key, buf.Bytes())
		buf.Reset()
	}
	return nil
}

// AccountState represents the state of a NEO account.
type AccountState struct {
	Version    uint8
	ScriptHash util.Uint160
	IsFrozen   bool
	Votes      []*keys.PublicKey
	Balances   map[util.Uint256]util.Fixed8
}

// NewAccountState returns a new AccountState object.
func NewAccountState(scriptHash util.Uint160) *AccountState {
	return &AccountState{
		Version:    0,
		ScriptHash: scriptHash,
		IsFrozen:   false,
		Votes:      []*keys.PublicKey{},
		Balances:   make(map[util.Uint256]util.Fixed8),
	}
}

// DecodeBinary decodes AccountState from the given io.Reader.
func (s *AccountState) DecodeBinary(r io.Reader) error {
	br := util.NewBinReaderFromIO(r)
	br.ReadLE(&s.Version)
	br.ReadLE(&s.ScriptHash)
	br.ReadLE(&s.IsFrozen)
	lenVotes := br.ReadVarUint()
	s.Votes = make([]*keys.PublicKey, lenVotes)
	for i := 0; i < int(lenVotes); i++ {
		s.Votes[i] = &keys.PublicKey{}
		if err := s.Votes[i].DecodeBinary(r); err != nil {
			return err
		}
	}

	s.Balances = make(map[util.Uint256]util.Fixed8)
	lenBalances := br.ReadVarUint()
	for i := 0; i < int(lenBalances); i++ {
		key := util.Uint256{}
		br.ReadLE(&key)
		var val util.Fixed8
		br.ReadLE(&val)
		s.Balances[key] = val
	}

	return br.Err
}

// EncodeBinary encode AccountState to the given io.Writer.
func (s *AccountState) EncodeBinary(w io.Writer) error {
	bw := util.NewBinWriterFromIO(w)
	bw.WriteLE(s.Version)
	bw.WriteLE(s.ScriptHash)
	bw.WriteLE(s.IsFrozen)
	bw.WriteVarUint(uint64(len(s.Votes)))
	for _, point := range s.Votes {
		if err := point.EncodeBinary(w); err != nil {
			return err
		}
	}

	balances := s.nonZeroBalances()
	bw.WriteVarUint(uint64(len(balances)))
	for k, v := range balances {
		bw.WriteLE(k)
		bw.WriteLE(v)
	}

	return bw.Err
}

// Returns only the non-zero balances for the account.
func (s *AccountState) nonZeroBalances() map[util.Uint256]util.Fixed8 {
	b := make(map[util.Uint256]util.Fixed8)
	for k, v := range s.Balances {
		if v > 0 {
			b[k] = v
		}
	}
	return b
}
