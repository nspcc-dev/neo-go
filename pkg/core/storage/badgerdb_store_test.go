package storage

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func newBadgerDBForTesting(t testing.TB) Store {
	bdbDir := t.TempDir()
	dbConfig := DBConfiguration{
		Type: "badgerdb",
		BadgerDBOptions: BadgerDBOptions{
			Dir: bdbDir,
		},
	}
	newBadgerStore, err := NewBadgerDBStore(dbConfig.BadgerDBOptions)
	require.Nil(t, err, "NewBadgerDBStore error")
	return newBadgerStore
}
