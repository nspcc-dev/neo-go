package storage

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func newBoltStoreForTesting(t testing.TB) Store {
	d := t.TempDir()
	testFileName := filepath.Join(d, "test_bolt_db")
	boltDBStore, err := NewBoltDBStore(BoltDBOptions{FilePath: testFileName})
	require.NoError(t, err)
	return boltDBStore
}
