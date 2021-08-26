package io

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakeDirForFile_HappyPath(t *testing.T) {
	tempDir := t.TempDir()
	filePath := path.Join(tempDir, "testDir/testFile.test")
	err := MakeDirForFile(filePath, "test")
	require.NoError(t, err)

	_, errChDir := os.Create(filePath)
	require.NoError(t, errChDir)
}

func TestMakeDirForFile_Negative(t *testing.T) {
	tempDir := t.TempDir()
	filePath := path.Join(tempDir, "testFile.test")
	_, err := os.Create(filePath)
	require.NoError(t, err)

	filePath = path.Join(filePath, "error")
	err = MakeDirForFile(filePath, "test")
	require.Errorf(t, err, "could not create dir for test: mkdir %s : not a directory", filePath)
}
