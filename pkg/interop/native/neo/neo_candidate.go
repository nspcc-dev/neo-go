package neo

import "github.com/nspcc-dev/neo-go/pkg/interop"

// Candidate represents a single native Neo candidate.
type Candidate struct {
	Key   interop.PublicKey
	Votes int
}
