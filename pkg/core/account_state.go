package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/crypto"
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
	Votes      []*crypto.PublicKey
	Balances   map[util.Uint256]util.Fixed8
}

// NewAccountState returns a new AccountState object.
func NewAccountState(scriptHash util.Uint160) *AccountState {
	return &AccountState{
		Version:    0,
		ScriptHash: scriptHash,
		IsFrozen:   false,
		Votes:      []*crypto.PublicKey{},
		Balances:   make(map[util.Uint256]util.Fixed8),
	}
}

// DecodeBinary decodes AccountState from the given io.Reader.
func (s *AccountState) DecodeBinary(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &s.Version); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &s.ScriptHash); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &s.IsFrozen); err != nil {
		return err
	}

	lenVotes := util.ReadVarUint(r)
	s.Votes = make([]*crypto.PublicKey, lenVotes)
	for i := 0; i < int(lenVotes); i++ {
		s.Votes[i] = &crypto.PublicKey{}
		if err := s.Votes[i].DecodeBinary(r); err != nil {
			return err
		}
	}

	s.Balances = make(map[util.Uint256]util.Fixed8)
	lenBalances := util.ReadVarUint(r)
	for i := 0; i < int(lenBalances); i++ {
		key := util.Uint256{}
		if err := binary.Read(r, binary.LittleEndian, &key); err != nil {
			return err
		}
		var val util.Fixed8
		if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
			return err
		}
		s.Balances[key] = val
	}

	return nil
}

// EncodeBinary encode AccountState to the given io.Writer.
func (s *AccountState) EncodeBinary(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, s.Version); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, s.ScriptHash); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, s.IsFrozen); err != nil {
		return err
	}

	if err := util.WriteVarUint(w, uint64(len(s.Votes))); err != nil {
		return err
	}
	for _, point := range s.Votes {
		if err := point.EncodeBinary(w); err != nil {
			return err
		}
	}

	balances := s.nonZeroBalances()
	if err := util.WriteVarUint(w, uint64(len(balances))); err != nil {
		return err
	}
	for k, v := range balances {
		if err := binary.Write(w, binary.LittleEndian, k); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, v); err != nil {
			return err
		}
	}

	return nil
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
