package io

import (
	"fmt"
	"os"
	"path/filepath"
)

// MakeDirForFile creates directory provided in filePath.
func MakeDirForFile(filePath string, creator string) error {
	fileName := filePath
	dir := filepath.Dir(fileName)
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create dir for %s: %w", creator, err)
	}
	return nil
}
