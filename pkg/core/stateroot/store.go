package stateroot

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

var (
	// ErrStateMismatch means that local state root doesn't match the one
	// signed by state validators.
	ErrStateMismatch = errors.New("stateroot mismatch")
)

const (
	prefixLocal     = 0x02
	prefixValidated = 0x03
)

func (s *Module) addLocalStateRoot(store *storage.MemCachedStore, sr *state.MPTRoot) error {
	key := makeStateRootKey(sr.Index)
	if err := putStateRoot(store, key, sr); err != nil {
		return err
	}

	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, sr.Index)
	return store.Put([]byte{byte(storage.DataMPT), prefixLocal}, data)
}

func putStateRoot(store *storage.MemCachedStore, key []byte, sr *state.MPTRoot) error {
	w := io.NewBufBinWriter()
	sr.EncodeBinary(w.BinWriter)
	return store.Put(key, w.Bytes())
}

func (s *Module) getStateRoot(key []byte) (*state.MPTRoot, error) {
	data, err := s.Store.Get(key)
	if err != nil {
		return nil, err
	}

	sr := &state.MPTRoot{}
	r := io.NewBinReaderFromBuf(data)
	sr.DecodeBinary(r)
	return sr, r.Err
}

func makeStateRootKey(index uint32) []byte {
	key := make([]byte, 5)
	key[0] = byte(storage.DataMPT)
	binary.BigEndian.PutUint32(key, index)
	return key
}

// AddStateRoot adds validated state root provided by network.
func (s *Module) AddStateRoot(sr *state.MPTRoot) error {
	if err := s.VerifyStateRoot(sr); err != nil {
		return err
	}
	key := makeStateRootKey(sr.Index)
	local, err := s.getStateRoot(key)
	if err != nil {
		return err
	}
	if !local.Root.Equals(sr.Root) {
		return fmt.Errorf("%w at block %d: %v vs %v", ErrStateMismatch, sr.Index, local.Root, sr.Root)
	}
	if len(local.Witness) != 0 {
		return nil
	}
	if err := putStateRoot(s.Store, key, sr); err != nil {
		return err
	}

	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, sr.Index)
	if err := s.Store.Put([]byte{byte(storage.DataMPT), prefixValidated}, data); err != nil {
		return err
	}
	s.validatedHeight.Store(sr.Index)
	if !s.bc.GetConfig().StateRootInHeader {
		updateStateHeightMetric(sr.Index)
	}
	return nil
}
