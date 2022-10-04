package storage

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/stretchr/testify/require"
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
