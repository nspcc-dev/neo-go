package main

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/util"
)

func TestUtilConvert(t *testing.T) {
	e := newExecutor(t, false)

	e.Run(t, "neo-go", "util", "convert", util.Uint160{1, 2, 3}.StringLE())
	e.checkNextLine(t, "f975")                                                                             // int to hex
	e.checkNextLine(t, "\\+XU=")                                                                           // int to base64
	e.checkNextLine(t, "NKuyBkoGdZZSLyPbJEetheRhMrGSCQx7YL")                                               // BE to address
	e.checkNextLine(t, "NL1JGiyJXdTkvFksXbFxgLJcWLj8Ewe7HW")                                               // LE to address
	e.checkNextLine(t, "Hex to String")                                                                    // hex to string
	e.checkNextLine(t, "5753853598078696051256155186041784866529345536")                                   // hex to int
	e.checkNextLine(t, "0102030000000000000000000000000000000000")                                         // swap endianness
	e.checkNextLine(t, "Base64 to String")                                                                 // base64 to string
	e.checkNextLine(t, "368753434210909009569191652203865891677393101439813372294890211308228051")         // base64 to bigint
	e.checkNextLine(t, "30303030303030303030303030303030303030303030303030303030303030303030303330323031") // string to hex
	e.checkNextLine(t, "MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAzMDIwMQ==")                         // string to base64
	e.checkEOF(t)
}
