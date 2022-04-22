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

func (s *Module) addLocalStateRoot(store *storage.MemCachedStore, sr *state.MPTRoot) {
	key := makeStateRootKey(sr.Index)
	putStateRoot(store, key, sr)

	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, sr.Index)
	store.Put([]byte{byte(storage.DataMPTAux), prefixLocal}, data)
}

func putStateRoot(store *storage.MemCachedStore, key []byte, sr *state.MPTRoot) {
	w := io.NewBufBinWriter()
	sr.EncodeBinary(w.BinWriter)
	store.Put(key, w.Bytes())
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
	key[0] = byte(storage.DataMPTAux)
	binary.BigEndian.PutUint32(key[1:], index)
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
	putStateRoot(s.Store, key, sr)

	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, sr.Index)
	s.Store.Put([]byte{byte(storage.DataMPTAux), prefixValidated}, data)
	s.validatedHeight.Store(sr.Index)
	if !s.srInHead {
		updateStateHeightMetric(sr.Index)
	}
	return nil
}
