package storage

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func newBoltStoreForTesting(t *testing.T) Store {
	testFileName := "test_bolt_db"
	file, err := ioutil.TempFile("", testFileName)
	t.Cleanup(func() {
		err := os.RemoveAll(testFileName)
		require.NoError(t, err)
	})
	require.NoError(t, err)
	require.NoError(t, file.Close())
	boltDBStore, err := NewBoltDBStore(BoltDBOptions{FilePath: testFileName})
	require.NoError(t, err)
	return boltDBStore
}
