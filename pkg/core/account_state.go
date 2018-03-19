package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// AccountStates is mapping between Uint160 and AccountState.
type AccountStates map[util.Uint160]*AccountState

func (s AccountStates) processTXOutputs(store storage.Store, outputs []*transaction.Output) error {
	for _, out := range outputs {
		if rawState, err := store.Get(out.ScriptHash.Bytes()); err != nil {
			// If the account state is not found create a new one.
			s[out.ScriptHash] = NewAccountState(out.ScriptHash)
		} else {
			fmt.Println("found this account state in the database !!")
			state := &AccountState{}
			if err := state.DecodeBinary(bytes.NewReader(rawState)); err != nil {
				return err
			}
			state.Balances[out.AssetID] += out.Amount
			s[state.ScriptHash] = state
		}
	}
	return nil
}

// Commit writes all account states to the given to Store.
func (s AccountStates) Commit(store storage.Store) error {
	buf := new(bytes.Buffer)
	for hash, state := range s {
		if err := state.EncodeBinary(buf); err != nil {
			return err
		}
		key := storage.AppendPrefix(storage.STAccount, hash.Bytes())
		if err := store.Put(key, buf.Bytes()); err != nil {
			return err
		}
		buf.Reset()
	}
	return nil
}

// AccountState represents the state of a NEO account.
type AccountState struct {
	ScriptHash util.Uint160
	IsFrozen   bool
	Votes      []*crypto.PublicKey
	Balances   map[util.Uint256]util.Fixed8
}

// NewAccountState returns a new AccountState object.
func NewAccountState(scriptHash util.Uint160) *AccountState {
	return &AccountState{
		ScriptHash: scriptHash,
		IsFrozen:   false,
		Votes:      []*crypto.PublicKey{},
		Balances:   make(map[util.Uint256]util.Fixed8),
	}
}

// DecodeBinary decodes AccountState from the given io.Reader.
func (s *AccountState) DecodeBinary(r io.Reader) error {
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

	if err := util.WriteVarUint(w, uint64(len(s.Balances))); err != nil {
		return err
	}

	for k, v := range s.Balances {
		if v > 0 {
			if err := binary.Write(w, binary.LittleEndian, k); err != nil {
				return err
			}
			if err := binary.Write(w, binary.LittleEndian, v); err != nil {
				return err
			}
		}
	}

	return nil
}
