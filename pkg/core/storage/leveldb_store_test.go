package storage

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func newLevelDBForTesting(t *testing.T) Store {
	ldbDir := t.TempDir()
	dbConfig := DBConfiguration{
		Type: "leveldb",
		LevelDBOptions: LevelDBOptions{
			DataDirectoryPath: ldbDir,
		},
	}
	newLevelStore, err := NewLevelDBStore(dbConfig.LevelDBOptions)
	require.Nil(t, err, "NewLevelDBStore error")
	return newLevelStore
}
