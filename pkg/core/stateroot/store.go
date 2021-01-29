package stateroot

import (
	"encoding/binary"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

const (
	prefixGC    = 0x01
	prefixLocal = 0x02
)

func (s *Module) addLocalStateRoot(sr *state.MPTRoot) error {
	key := makeStateRootKey(sr.Index)
	if err := s.putStateRoot(key, sr); err != nil {
		return err
	}

	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, sr.Index)
	if err := s.Store.Put([]byte{byte(storage.DataMPT), prefixLocal}, data); err != nil {
		return err
	}
	s.currentLocal.Store(sr.Root)
	s.localHeight.Store(sr.Index)
	updateStateHeightMetric(sr.Index)
	return nil
}

func (s *Module) putStateRoot(key []byte, sr *state.MPTRoot) error {
	w := io.NewBufBinWriter()
	sr.EncodeBinary(w.BinWriter)
	return s.Store.Put(key, w.Bytes())
}

func (s *Module) getStateRoot(key []byte) (*state.MPTRoot, error) {
	data, err := s.Store.Get(key)
	if err != nil {
		return nil, err
	}

	sr := new(state.MPTRoot)
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
