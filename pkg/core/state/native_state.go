package state

import (
	"crypto/elliptic"
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// NEP17Balance represents balance state of a NEP17-token.
type NEP17Balance struct {
	Balance big.Int
}

// NEOBalance represents balance state of a NEO-token.
type NEOBalance struct {
	NEP17Balance
	BalanceHeight uint32
	VoteTo        *keys.PublicKey
}

// NEP17BalanceFromBytes converts serialized NEP17Balance to structure.
func NEP17BalanceFromBytes(b []byte) (*NEP17Balance, error) {
	balance := new(NEP17Balance)
	err := balanceFromBytes(b, balance)
	if err != nil {
		return nil, err
	}
	return balance, nil
}

// Bytes returns serialized NEP17Balance.
func (s *NEP17Balance) Bytes() []byte {
	return balanceToBytes(s)
}

func balanceFromBytes(b []byte, item stackitem.Convertible) error {
	if len(b) == 0 {
		return nil
	}
	return stackitem.DeserializeConvertible(b, item)
}

func balanceToBytes(item stackitem.Convertible) []byte {
	data, err := stackitem.SerializeConvertible(item)
	if err != nil {
		panic(err)
	}
	return data
}

// ToStackItem implements stackitem.Convertible. It never returns an error.
func (s *NEP17Balance) ToStackItem() (stackitem.Item, error) {
	return stackitem.NewStruct([]stackitem.Item{stackitem.NewBigInteger(&s.Balance)}), nil
}

// FromStackItem implements stackitem.Convertible.
func (s *NEP17Balance) FromStackItem(item stackitem.Item) error {
	items, ok := item.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not a struct")
	}
	if len(items) < 1 {
		return errors.New("no balance value")
	}
	balance, err := items[0].TryInteger()
	if err != nil {
		return fmt.Errorf("invalid balance: %w", err)
	}
	s.Balance = *balance
	return nil
}

// NEOBalanceFromBytes converts serialized NEOBalance to structure.
func NEOBalanceFromBytes(b []byte) (*NEOBalance, error) {
	balance := new(NEOBalance)
	err := balanceFromBytes(b, balance)
	if err != nil {
		return nil, err
	}
	return balance, nil
}

// Bytes returns serialized NEOBalance.
func (s *NEOBalance) Bytes() []byte {
	return balanceToBytes(s)
}

// ToStackItem implements stackitem.Convertible interface. It never returns an error.
func (s *NEOBalance) ToStackItem() (stackitem.Item, error) {
	var voteItem stackitem.Item

	if s.VoteTo != nil {
		voteItem = stackitem.NewByteArray(s.VoteTo.Bytes())
	} else {
		voteItem = stackitem.Null{}
	}
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.NewBigInteger(&s.Balance),
		stackitem.NewBigInteger(big.NewInt(int64(s.BalanceHeight))),
		voteItem,
	}), nil
}

// FromStackItem converts stackitem.Item to NEOBalance.
func (s *NEOBalance) FromStackItem(item stackitem.Item) error {
	structItem, ok := item.Value().([]stackitem.Item)
	if !ok || len(structItem) < 3 {
		return errors.New("invalid stackitem length")
	}
	balance, err := structItem[0].TryInteger()
	if err != nil {
		return fmt.Errorf("invalid balance stackitem: %w", err)
	}
	s.Balance = *balance
	h, err := structItem[1].TryInteger()
	if err != nil {
		return fmt.Errorf("invalid heigh stackitem")
	}
	s.BalanceHeight = uint32(h.Int64())
	if _, ok := structItem[2].(stackitem.Null); ok {
		s.VoteTo = nil
		return nil
	}
	bs, err := structItem[2].TryBytes()
	if err != nil {
		return fmt.Errorf("invalid public key stackitem: %w", err)
	}
	pub, err := keys.NewPublicKeyFromBytes(bs, elliptic.P256())
	if err != nil {
		return fmt.Errorf("invalid public key bytes: %w", err)
	}
	s.VoteTo = pub
	return nil
}
