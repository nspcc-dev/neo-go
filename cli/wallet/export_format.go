package wallet

import "fmt"

// exportFormat is a type used to specify the format of an exported private key.
type exportFormat byte

const (
	exportNEP2 exportFormat = iota
	exportWIF
	exportPEM
)

func exportFormatFromString(s string) (exportFormat, error) {
	switch s {
	case "nep2":
		return exportNEP2, nil
	case "wif":
		return exportWIF, nil
	case "pem":
		return exportPEM, nil
	default:
		return 0, fmt.Errorf("unknown private key export format: %s", s)
	}
}
