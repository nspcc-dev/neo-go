package native

import (
	"crypto/elliptic"
	"errors"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// gasIndexPair contains block index together with generated gas per block.
// It is used to cache NEO GASRecords.
type gasIndexPair struct {
	Index       uint32
	GASPerBlock big.Int
}

// gasRecord contains history of gas per block changes. It is used only by NEO cache.
type gasRecord []gasIndexPair

type (
	// keyWithVotes is a serialized key with votes balance. It's not deserialized
	// because some uses of it imply serialized-only usage and converting to
	// PublicKey is quite expensive.
	keyWithVotes struct {
		Key   string
		Votes *big.Int
		// UnmarshaledKey contains public key if it was unmarshaled.
		UnmarshaledKey *keys.PublicKey
	}

	keysWithVotes []keyWithVotes
)

// PublicKey unmarshals and returns public key of k.
func (k keyWithVotes) PublicKey() (*keys.PublicKey, error) {
	if k.UnmarshaledKey != nil {
		return k.UnmarshaledKey, nil
	}
	return keys.NewPublicKeyFromBytes([]byte(k.Key), elliptic.P256())
}

func (k keysWithVotes) toStackItem() stackitem.Item {
	arr := make([]stackitem.Item, len(k))
	for i := range k {
		arr[i] = stackitem.NewStruct([]stackitem.Item{
			stackitem.NewByteArray([]byte(k[i].Key)),
			stackitem.NewBigInteger(k[i].Votes),
		})
	}
	return stackitem.NewArray(arr)
}

func (k *keysWithVotes) fromStackItem(item stackitem.Item) error {
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not an array")
	}

	var kvs = make(keysWithVotes, len(arr))
	for i := range arr {
		s, ok := arr[i].Value().([]stackitem.Item)
		if !ok {
			return errors.New("element is not a struct")
		} else if len(s) < 2 {
			return errors.New("invalid length")
		}
		pub, err := s[0].TryBytes()
		if err != nil {
			return err
		}
		vs, err := s[1].TryInteger()
		if err != nil {
			return err
		}
		kvs[i].Key = string(pub)
		kvs[i].Votes = vs
	}
	*k = kvs
	return nil
}

// Bytes serializes keys with votes slice.
func (k keysWithVotes) Bytes() []byte {
	buf, err := stackitem.Serialize(k.toStackItem())
	if err != nil {
		panic(err)
	}
	return buf
}

// DecodeBytes deserializes keys and votes slice.
func (k *keysWithVotes) DecodeBytes(data []byte) error {
	it, err := stackitem.Deserialize(data)
	if err != nil {
		return err
	}
	return k.fromStackItem(it)
}
