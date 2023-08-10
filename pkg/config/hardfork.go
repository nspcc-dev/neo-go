package config

//go:generate stringer -type=Hardfork -linecomment

// Hardfork represents the application hard-fork identifier.
type Hardfork byte

const (
	// HFAspidochelone represents hard-fork introduced in #2469 (ported from
	// https://github.com/neo-project/neo/pull/2712) and #2519 (ported from
	// https://github.com/neo-project/neo/pull/2749).
	HFAspidochelone Hardfork = 1 << iota // Aspidochelone
	// HFBasilisk represents hard-fork introduced in #3056 (ported from
	// https://github.com/neo-project/neo/pull/2881), #3080 (ported from
	// https://github.com/neo-project/neo/pull/2883) and #3085 (ported from
	// https://github.com/neo-project/neo/pull/2810).
	HFBasilisk // Basilisk
)

// hardforks holds a map of Hardfork string representation to its type.
var hardforks map[string]Hardfork

func init() {
	hardforks = make(map[string]Hardfork)
	for _, hf := range []Hardfork{HFAspidochelone, HFBasilisk} {
		hardforks[hf.String()] = hf
	}
}

// IsHardforkValid denotes whether the provided string represents a valid
// Hardfork name.
func IsHardforkValid(s string) bool {
	_, ok := hardforks[s]
	return ok
}
