package store

import (
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/stretchr/testify/require"
)

// NewLevelDBForTesting creates a new LevelDB store in a TempDir.
func NewLevelDBForTesting(t testing.TB) storage.Store {
	s, _ := NewLevelDBForTestingWithPath(t, "")
	return s
}

// NewLevelDBForTestingWithPath creates a new LevelDB store localized in the
// specified directory (TempDir is used if not specified).
func NewLevelDBForTestingWithPath(t testing.TB, dbPath string) (storage.Store, string) {
	if dbPath == "" {
		dbPath = t.TempDir()
	}
	dbOptions := dbconfig.LevelDBOptions{
		DataDirectoryPath: dbPath,
	}
	newLevelStore, err := storage.NewLevelDBStore(dbOptions)
	require.NoError(t, err, "failed to create LevelDBStore for testing")
	return newLevelStore, dbPath
}

// NewBoltDBForTesting creates a new BoltDB store in a TempDir.
func NewBoltDBForTesting(t testing.TB) storage.Store {
	s, _ := NewBoltDBForTestingWithPath(t, "")
	return s
}

// NewBoltDBForTestingWithPath creates a new BoltDB store localized in the
// specified directory (TempDir is used if not specified).
func NewBoltDBForTestingWithPath(t testing.TB, dbPath string) (storage.Store, string) {
	if dbPath == "" {
		dbPath = filepath.Join(t.TempDir(), "test_bolt_db")
	}
	boltDBStore, err := storage.NewBoltDBStore(dbconfig.BoltDBOptions{FilePath: dbPath})
	require.NoError(t, err, "failed to create BoltDBStore for testing")
	return boltDBStore, dbPath
}
