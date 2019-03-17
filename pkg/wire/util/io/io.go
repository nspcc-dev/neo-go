package fileutils

import (
	"os"
)

// UpdateFile appends a byte slice to a file
func UpdateFile(filename string, data []byte) error {

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	dataWNewline := append(data, []byte("\n")...)

	_, err = f.Write(dataWNewline)
	err = f.Close()
	return err
}
