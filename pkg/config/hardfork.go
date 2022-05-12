package config

//go:generate stringer -type=Hardfork -linecomment

// Hardfork represents the application hard-fork identifier.
type Hardfork byte

const (
	// HF2712FixSyscallFees represents hard-fork introduced in #2469 (ported from
	// https://github.com/neo-project/neo/pull/2712) changing the prices of
	// System.Contract.CreateStandardAccount and
	// System.Contract.CreateMultisigAccount interops.
	HF2712FixSyscallFees Hardfork = 1 << iota // HF_2712_FixSyscallFees
)

// hardforks holds a map of Hardfork string representation to its type.
var hardforks map[string]Hardfork

func init() {
	hardforks = make(map[string]Hardfork)
	for _, hf := range []Hardfork{HF2712FixSyscallFees} {
		hardforks[hf.String()] = hf
	}
}

// IsHardforkValid denotes whether the provided string represents a valid
// Hardfork name.
func IsHardforkValid(s string) bool {
	_, ok := hardforks[s]
	return ok
}
