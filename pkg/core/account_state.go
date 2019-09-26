package core

import (
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// Accounts is mapping between a account address and AccountState.
type Accounts map[util.Uint160]*AccountState

// getAndUpdate retrieves AccountState from temporary or persistent Store
// or creates a new one if it doesn't exist.
func (a Accounts) getAndUpdate(ts storage.Store, ps storage.Store, hash util.Uint160) (*AccountState, error) {
	if account, ok := a[hash]; ok {
		return account, nil
	}

	account, err := getAccountStateFromStore(ts, hash)
	if err != nil {
		if err != storage.ErrKeyNotFound {
			return nil, err
		}
		account, err = getAccountStateFromStore(ps, hash)
		if err != nil {
			if err != storage.ErrKeyNotFound {
				return nil, err
			}
			account = NewAccountState(hash)
		}
	}

	a[hash] = account
	return account, nil
}

// getAccountStateFromStore returns AccountState from the given Store if it's
// present there. Returns nil otherwise.
func getAccountStateFromStore(s storage.Store, hash util.Uint160) (*AccountState, error) {
	var account *AccountState
	key := storage.AppendPrefix(storage.STAccount, hash.Bytes())
	b, err := s.Get(key)
	if err == nil {
		account = new(AccountState)
		r := io.NewBinReaderFromBuf(b)
		account.DecodeBinary(r)
		if r.Err != nil {
			return nil, fmt.Errorf("failed to decode (AccountState): %s", r.Err)
		}
	}
	return account, err
}

// commit writes all account states to the given Batch.
func (a Accounts) commit(b storage.Batch) error {
	buf := io.NewBufBinWriter()
	for hash, state := range a {
		state.EncodeBinary(buf.BinWriter)
		if buf.Err != nil {
			return buf.Err
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

// DecodeBinary decodes AccountState from the given BinReader.
func (s *AccountState) DecodeBinary(br *io.BinReader) {
	br.ReadLE(&s.Version)
	br.ReadLE(&s.ScriptHash)
	br.ReadLE(&s.IsFrozen)
	lenVotes := br.ReadVarUint()
	s.Votes = make([]*keys.PublicKey, lenVotes)
	for i := 0; i < int(lenVotes); i++ {
		s.Votes[i] = &keys.PublicKey{}
		s.Votes[i].DecodeBinary(br)
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
}

// EncodeBinary encodes AccountState to the given BinWriter.
func (s *AccountState) EncodeBinary(bw *io.BinWriter) {
	bw.WriteLE(s.Version)
	bw.WriteLE(s.ScriptHash)
	bw.WriteLE(s.IsFrozen)
	bw.WriteVarUint(uint64(len(s.Votes)))
	for _, point := range s.Votes {
		point.EncodeBinary(bw)
	}

	balances := s.nonZeroBalances()
	bw.WriteVarUint(uint64(len(balances)))
	for k, v := range balances {
		bw.WriteLE(k)
		bw.WriteLE(v)
	}
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
