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
func (a Accounts) getAndUpdate(s storage.Store, hash util.Uint160) (*AccountState, error) {
	if account, ok := a[hash]; ok {
		return account, nil
	}

	account, err := getAccountStateFromStore(s, hash)
	if err != nil {
		if err != storage.ErrKeyNotFound {
			return nil, err
		}
		account = NewAccountState(hash)
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

// putAccountStateIntoStore puts given AccountState into the given store.
func putAccountStateIntoStore(store storage.Store, as *AccountState) error {
	buf := io.NewBufBinWriter()
	as.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	key := storage.AppendPrefix(storage.STAccount, as.ScriptHash.Bytes())
	return store.Put(key, buf.Bytes())
}

// commit writes all account states to the given Batch.
func (a Accounts) commit(store storage.Store) error {
	for _, state := range a {
		if err := putAccountStateIntoStore(store, state); err != nil {
			return err
		}
	}
	return nil
}

// UnspentBalance contains input/output transactons that sum up into the
// account balance for the given asset.
type UnspentBalance struct {
	Tx    util.Uint256 `json:"txid"`
	Index uint16       `json:"n"`
	Value util.Fixed8  `json:"value"`
}

// UnspentBalances is a slice of UnspentBalance (mostly needed to sort them).
type UnspentBalances []UnspentBalance

// AccountState represents the state of a NEO account.
type AccountState struct {
	Version    uint8
	ScriptHash util.Uint160
	IsFrozen   bool
	Votes      []*keys.PublicKey
	Balances   map[util.Uint256][]UnspentBalance
}

// NewAccountState returns a new AccountState object.
func NewAccountState(scriptHash util.Uint160) *AccountState {
	return &AccountState{
		Version:    0,
		ScriptHash: scriptHash,
		IsFrozen:   false,
		Votes:      []*keys.PublicKey{},
		Balances:   make(map[util.Uint256][]UnspentBalance),
	}
}

// DecodeBinary decodes AccountState from the given BinReader.
func (s *AccountState) DecodeBinary(br *io.BinReader) {
	br.ReadLE(&s.Version)
	br.ReadLE(&s.ScriptHash)
	br.ReadLE(&s.IsFrozen)
	br.ReadArray(&s.Votes)

	s.Balances = make(map[util.Uint256][]UnspentBalance)
	lenBalances := br.ReadVarUint()
	for i := 0; i < int(lenBalances); i++ {
		key := util.Uint256{}
		br.ReadLE(&key)
		ubs := make([]UnspentBalance, 0)
		br.ReadArray(&ubs)
		s.Balances[key] = ubs
	}
}

// EncodeBinary encodes AccountState to the given BinWriter.
func (s *AccountState) EncodeBinary(bw *io.BinWriter) {
	bw.WriteLE(s.Version)
	bw.WriteBytes(s.ScriptHash[:])
	bw.WriteLE(s.IsFrozen)
	bw.WriteArray(s.Votes)

	bw.WriteVarUint(uint64(len(s.Balances)))
	for k, v := range s.Balances {
		bw.WriteBytes(k[:])
		bw.WriteArray(v)
	}
}

// DecodeBinary implements io.Serializable interface.
func (u *UnspentBalance) DecodeBinary(r *io.BinReader) {
	u.Tx.DecodeBinary(r)
	r.ReadLE(&u.Index)
	r.ReadLE(&u.Value)
}

// EncodeBinary implements io.Serializable interface.
func (u UnspentBalance) EncodeBinary(w *io.BinWriter) {
	u.Tx.EncodeBinary(w)
	w.WriteLE(u.Index)
	w.WriteLE(u.Value)
}

// GetBalanceValues sums all unspent outputs and returns a map of asset IDs to
// overall balances.
func (s *AccountState) GetBalanceValues() map[util.Uint256]util.Fixed8 {
	res := make(map[util.Uint256]util.Fixed8)
	for k, v := range s.Balances {
		balance := util.Fixed8(0)
		for _, b := range v {
			balance += b.Value
		}
		res[k] = balance
	}
	return res
}

// Len returns the length of UnspentBalances (used to sort things).
func (us UnspentBalances) Len() int { return len(us) }

// Less compares two elements of UnspentBalances (used to sort things).
func (us UnspentBalances) Less(i, j int) bool { return us[i].Value < us[j].Value }

// Swap swaps two elements of UnspentBalances (used to sort things).
func (us UnspentBalances) Swap(i, j int) { us[i], us[j] = us[j], us[i] }
