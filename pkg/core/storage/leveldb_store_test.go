package storage

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/stretchr/testify/require"
)

func newLevelDBForTesting(t testing.TB) Store {
	ldbDir := t.TempDir()
	dbConfig := dbconfig.DBConfiguration{
		Type: "leveldb",
		LevelDBOptions: dbconfig.LevelDBOptions{
			DataDirectoryPath: ldbDir,
		},
	}
	newLevelStore, err := NewLevelDBStore(dbConfig.LevelDBOptions)
	require.Nil(t, err, "NewLevelDBStore error")
	return newLevelStore
}
