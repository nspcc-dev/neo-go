package transaction

import (
	"errors"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/version"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

// Issue represents an issue transaction on the neo network
type Issue struct {
	*Base
}

//NewIssue returns an issue transaction
func NewIssue(ver version.TX) *Issue {
	basicTrans := createBaseTransaction(types.Issue, ver)

	Issue := &Issue{
		basicTrans,
	}
	Issue.encodeExclusive = Issue.encodeExcl
	Issue.decodeExclusive = Issue.decodeExcl
	return Issue
}

func (c *Issue) encodeExcl(bw *util.BinWriter) {
	if c.Version > 1 {
		bw.Err = errors.New("Version Number Invalid, Issue cannot be more than 0")
	}
}

func (c *Issue) decodeExcl(br *util.BinReader) {}
