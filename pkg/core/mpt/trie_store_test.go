package mpt

import (
	"bytes"
	"errors"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/stretchr/testify/require"
)

func TestTrieStore_TestTrieOperations(t *testing.T) {
	source := newTestTrie(t)
	backed := source.Store

	st := NewTrieStore(source.root.Hash(), ModeAll, backed)

	t.Run("forbidden operations", func(t *testing.T) {
		require.ErrorIs(t, st.SeekGC(storage.SeekRange{}, nil), errors.ErrUnsupported)
		_, err := st.Get([]byte{byte(storage.STTokenTransferInfo)})
		require.ErrorIs(t, err, errors.ErrUnsupported)
		require.ErrorIs(t, st.PutChangeSet(nil, nil), errors.ErrUnsupported)
	})

	t.Run("Get", func(t *testing.T) {
		t.Run("good", func(t *testing.T) {
			res, err := st.Get(append([]byte{byte(storage.STStorage)}, 0xAC, 0xae)) // leaf `hello`
			require.NoError(t, err)
			require.Equal(t, []byte("hello"), res)
		})
		t.Run("bad path", func(t *testing.T) {
			_, err := st.Get(append([]byte{byte(storage.STStorage)}, 0xAC, 0xa0)) // bad path
			require.ErrorIs(t, err, storage.ErrKeyNotFound)
		})
		t.Run("path to not-a-leaf", func(t *testing.T) {
			_, err := st.Get(append([]byte{byte(storage.STStorage)}, 0xAC)) // path to extension
			require.ErrorIs(t, err, storage.ErrKeyNotFound)
		})
	})

	t.Run("Seek", func(t *testing.T) {
		check := func(t *testing.T, backwards bool) {
			var res [][]byte
			st.Seek(storage.SeekRange{
				Prefix:    []byte{byte(storage.STStorage)},
				Start:     nil,
				Backwards: backwards,
			}, func(k, v []byte) bool {
				res = append(res, k)
				return true
			})
			require.Equal(t, 4, len(res))
			for i := range res {
				require.Equal(t, byte(storage.STStorage), res[i][0])
				if i < len(res)-1 {
					cmp := bytes.Compare(res[i], res[i+1])
					if backwards {
						require.True(t, cmp > 0)
					} else {
						require.True(t, cmp < 0)
					}
				}
			}
		}
		t.Run("good: over whole storage", func(t *testing.T) {
			t.Run("forwards", func(t *testing.T) {
				check(t, false)
			})
			t.Run("backwards", func(t *testing.T) {
				check(t, true)
			})
		})
	})
}
