package config

//go:generate stringer -type=Hardfork -linecomment

// Hardfork represents the application hard-fork identifier.
type Hardfork byte

// HFDefault is a default value of Hardfork enum. It's a special constant
// aimed to denote the node code enabled by default starting from the
// genesis block. HFDefault is not a hard-fork, but this constant can be used for
// convenient hard-forks comparison and to refer to the default hard-fork-less
// node behaviour.
const HFDefault Hardfork = 0 // Default

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
	// HFCockatrice represents hard-fork introduced in #3402 (ported from
	// https://github.com/neo-project/neo/pull/2942), #3301 (ported from
	// https://github.com/neo-project/neo/pull/2925) and #3362 (ported from
	// https://github.com/neo-project/neo/pull/3154).
	HFCockatrice // Cockatrice
	// hfLast denotes the end of hardforks enum. Consider adding new hardforks
	// before hfLast.
	hfLast
)

// Hardforks represents the ordered slice of all possible hardforks.
var Hardforks []Hardfork

// hardforks holds a map of Hardfork string representation to its type.
var hardforks map[string]Hardfork

func init() {
	for i := HFAspidochelone; i < hfLast; i = i << 1 {
		Hardforks = append(Hardforks, i)
	}
	hardforks = make(map[string]Hardfork, len(Hardforks))
	for _, hf := range Hardforks {
		hardforks[hf.String()] = hf
	}
}

// Cmp returns the result of hardforks comparison. It returns:
//
//	-1 if hf <  other
//	 0 if hf == other
//	+1 if hf >  other
func (hf Hardfork) Cmp(other Hardfork) int {
	switch {
	case hf == other:
		return 0
	case hf < other:
		return -1
	default:
		return 1
	}
}

// Prev returns the previous hardfork for the given one. Calling Prev for the default hardfork is a no-op.
func (hf Hardfork) Prev() Hardfork {
	if hf == HFDefault {
		panic("unexpected call to Prev for the default hardfork")
	}
	return hf >> 1
}

// IsHardforkValid denotes whether the provided string represents a valid
// Hardfork name.
func IsHardforkValid(s string) bool {
	_, ok := hardforks[s]
	return ok
}

// LatestHardfork returns latest known hardfork.
func LatestHardfork() Hardfork {
	return hfLast >> 1
}
