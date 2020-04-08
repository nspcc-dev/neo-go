package storage

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type tempBadgerDB struct {
	*BadgerDBStore
	dir string
}

func (tbdb *tempBadgerDB) Close() error {
	err := tbdb.BadgerDBStore.Close()
	// Make test fail if failed to cleanup, even though technically it's
	// not a BadgerDBStore problem.
	osErr := os.RemoveAll(tbdb.dir)
	if osErr != nil {
		return osErr
	}
	return err
}

func newBadgerDBForTesting(t *testing.T) Store {
	bdbDir, err := ioutil.TempDir(os.TempDir(), "testbadgerdb")
	require.Nil(t, err, "failed to setup temporary directory")

	dbConfig := DBConfiguration{
		Type: "badgerdb",
		BadgerDBOptions: BadgerDBOptions{
			Dir: bdbDir,
		},
	}
	newBadgerStore, err := NewBadgerDBStore(dbConfig.BadgerDBOptions)
	require.Nil(t, err, "NewBadgerDBStore error")
	tbdb := &tempBadgerDB{
		BadgerDBStore: newBadgerStore,
		dir:           bdbDir,
	}
	return tbdb
}
