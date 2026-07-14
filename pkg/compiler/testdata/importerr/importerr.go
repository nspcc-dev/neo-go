package importerr

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/native/std"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

type Ballot struct {
	X int
}

// GetBallots contains an invalid type assertion: storage.LocalGet already
// returns []byte, so data.([]byte) doesn't type-check (data is not an
// interface). This must be reported as a normal compile error, not silently
// ignored because the error is in an imported package.
func GetBallots() []Ballot {
	data := storage.LocalGet([]byte("votekey"))
	if data != nil {
		return std.Deserialize(data.([]byte)).([]Ballot)
	}
	return []Ballot{}
}
