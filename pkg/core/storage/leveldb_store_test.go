package storage

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/stretchr/testify/require"
	"github.com/syndtr/goleveldb/leveldb"
)

func newLevelDBForTesting(t testing.TB) Store {
	ldbDir := t.TempDir()
	opts := dbconfig.LevelDBOptions{
		DataDirectoryPath: ldbDir,
	}
	newLevelStore, err := NewLevelDBStore(opts)
	require.Nil(t, err, "NewLevelDBStore error")
	return newLevelStore
}

func TestROLevelDB(t *testing.T) {
	ldbDir := t.TempDir()
	opts := dbconfig.LevelDBOptions{
		DataDirectoryPath: ldbDir,
		ReadOnly:          true,
	}

	// If DB doesn't exist, then error should be returned.
	_, err := NewLevelDBStore(opts)
	require.Error(t, err)

	// Create the DB and try to open it in RO mode.
	opts.ReadOnly = false
	store, err := NewLevelDBStore(opts)
	require.NoError(t, err)
	require.NoError(t, store.Close())
	opts.ReadOnly = true

	store, err = NewLevelDBStore(opts)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	// Changes must be prohibited.
	putErr := store.PutChangeSet(map[string][]byte{"one": []byte("one")}, nil)
	require.ErrorIs(t, putErr, leveldb.ErrReadOnly)
}
