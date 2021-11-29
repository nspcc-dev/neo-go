package io

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakeDirForFile_HappyPath(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "testDir", "testFile.test")
	err := MakeDirForFile(filePath, "test")
	require.NoError(t, err)

	f, errChDir := os.Create(filePath)
	require.NoError(t, errChDir)
	require.NoError(t, f.Close())
}

func TestMakeDirForFile_Negative(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "testFile.test")
	f, err := os.Create(filePath)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	filePath = filepath.Join(filePath, "error")
	err = MakeDirForFile(filePath, "test")
	require.Errorf(t, err, "could not create dir for test: mkdir %s : not a directory", filePath)
}
