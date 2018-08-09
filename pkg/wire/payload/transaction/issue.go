package transaction

import (
	"errors"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/version"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type Issue struct {
	*Base
}

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
	return
}

func (c *Issue) decodeExcl(br *util.BinReader) {
	return
}
