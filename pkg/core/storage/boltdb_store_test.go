package storage

import (
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/stretchr/testify/require"
)

func newBoltStoreForTesting(t testing.TB) Store {
	d := t.TempDir()
	testFileName := filepath.Join(d, "test_bolt_db")
	boltDBStore, err := NewBoltDBStore(dbconfig.BoltDBOptions{FilePath: testFileName})
	require.NoError(t, err)
	return boltDBStore
}
