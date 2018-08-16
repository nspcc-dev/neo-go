package fileutils

import (
	"os"
)

func UpdateFile(filename string, data []byte) error {

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	dataWNewline := append(data, []byte("\n")...)

	_, err = f.Write(dataWNewline)
	err = f.Close()
	return err
}
