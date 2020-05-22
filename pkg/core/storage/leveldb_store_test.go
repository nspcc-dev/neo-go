package storage

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type tempLevelDB struct {
	LevelDBStore
	dir string
}

func (tldb *tempLevelDB) Close() error {
	err := tldb.LevelDBStore.Close()
	// Make test fail if failed to cleanup, even though technically it's
	// not a LevelDBStore problem.
	osErr := os.RemoveAll(tldb.dir)
	if osErr != nil {
		return osErr
	}
	return err
}

func newLevelDBForTesting(t *testing.T) Store {
	ldbDir, err := ioutil.TempDir(os.TempDir(), "testleveldb")
	require.Nil(t, err, "failed to setup temporary directory")

	dbConfig := DBConfiguration{
		Type: "leveldb",
		LevelDBOptions: LevelDBOptions{
			DataDirectoryPath: ldbDir,
		},
	}
	newLevelStore, err := NewLevelDBStore(dbConfig.LevelDBOptions)
	require.Nil(t, err, "NewLevelDBStore error")
	tldb := &tempLevelDB{LevelDBStore: *newLevelStore, dir: ldbDir}
	return tldb
}
