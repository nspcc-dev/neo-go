package server

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetPath(t *testing.T) {
	testPath := t.TempDir()
	actual, err := getPath(testPath, 123)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(testPath, "BlockStorage_100000", "dump-block-1000.json"), actual)

	actual, err = getPath(testPath, 1230)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(testPath, "BlockStorage_100000", "dump-block-2000.json"), actual)

	actual, err = getPath(testPath, 123000)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(testPath, "BlockStorage_200000", "dump-block-123000.json"), actual)
}
