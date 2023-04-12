package util_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testcli"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestUtilConvert(t *testing.T) {
	e := testcli.NewExecutor(t, false)

	e.Run(t, "neo-go", "util", "convert", util.Uint160{1, 2, 3}.StringLE())
	e.CheckNextLine(t, "f975")                                                                             // int to hex
	e.CheckNextLine(t, "\\+XU=")                                                                           // int to base64
	e.CheckNextLine(t, "NKuyBkoGdZZSLyPbJEetheRhMrGSCQx7YL")                                               // BE to address
	e.CheckNextLine(t, "NL1JGiyJXdTkvFksXbFxgLJcWLj8Ewe7HW")                                               // LE to address
	e.CheckNextLine(t, "Hex to String")                                                                    // hex to string
	e.CheckNextLine(t, "5753853598078696051256155186041784866529345536")                                   // hex to int
	e.CheckNextLine(t, "0102030000000000000000000000000000000000")                                         // swap endianness
	e.CheckNextLine(t, "Base64 to String")                                                                 // base64 to string
	e.CheckNextLine(t, "368753434210909009569191652203865891677393101439813372294890211308228051")         // base64 to bigint
	e.CheckNextLine(t, "30303030303030303030303030303030303030303030303030303030303030303030303330323031") // string to hex
	e.CheckNextLine(t, "MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAzMDIwMQ==")                         // string to base64
	e.CheckEOF(t)
}

func TestUtilOps(t *testing.T) {
	e := testcli.NewExecutor(t, false)
	base64Str := "EUA="
	hexStr := "1140"

	check := func(t *testing.T) {
		e.CheckNextLine(t, "INDEX.*OPCODE.*PARAMETER")
		e.CheckNextLine(t, "PUSH1")
		e.CheckNextLine(t, "RET")
		e.CheckEOF(t)
	}

	e.Run(t, "neo-go", "util", "ops", base64Str) // base64
	check(t)

	e.Run(t, "neo-go", "util", "ops", hexStr) // base64 is checked firstly by default, but it's invalid script if decoded from base64
	e.CheckNextLine(t, "INDEX.*OPCODE.*PARAMETER")
	e.CheckNextLine(t, ".*ERROR: incorrect opcode")
	e.CheckEOF(t)

	e.Run(t, "neo-go", "util", "ops", "--hex", hexStr) // explicitly specify hex encoding
	check(t)

	e.RunWithError(t, "neo-go", "util", "ops", "%&~*") // unknown encoding

	tmp := filepath.Join(t.TempDir(), "script_base64.txt")
	require.NoError(t, os.WriteFile(tmp, []byte(base64Str), os.ModePerm))
	e.Run(t, "neo-go", "util", "ops", "--in", tmp) // base64 from file
	check(t)

	tmp = filepath.Join(t.TempDir(), "script_hex.txt")
	require.NoError(t, os.WriteFile(tmp, []byte(hexStr), os.ModePerm))
	e.Run(t, "neo-go", "util", "ops", "--hex", "--in", tmp) // hex from file
	check(t)
}
