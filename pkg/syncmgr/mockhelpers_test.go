package syncmgr

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type syncTestHelper struct {
	blocksProcessed     int
	headersProcessed    int
	newBlockRequest     int
	headersFetchRequest int
	blockFetchRequest   int
	err                 error
}

func (s *syncTestHelper) ProcessBlock(msg payload.Block) error {
	s.blocksProcessed++
	return s.err
}
func (s *syncTestHelper) ProcessHeaders(hdrs []*payload.BlockBase) error {
	s.headersProcessed = s.headersProcessed + len(hdrs)
	return s.err
}

func (s *syncTestHelper) GetNextBlockHash() (util.Uint256, error) {
	return util.Uint256{}, s.err
}

func (s *syncTestHelper) AskForNewBlocks() {
	s.newBlockRequest++
}

func (s *syncTestHelper) FetchHeadersAgain(util.Uint256) error {
	s.headersFetchRequest++
	return s.err
}

func (s *syncTestHelper) FetchBlockAgain(util.Uint256) error {
	s.blockFetchRequest++
	return s.err
}

type mockPeer struct {
	height uint32
}

func (p *mockPeer) RequestBlocks(hashes []util.Uint256) error { return nil }
func (p *mockPeer) RequestHeaders(hash util.Uint256) error    { return nil }
func (p *mockPeer) Height() uint32                            { return p.height }

func randomHeadersMessage(t *testing.T, num int) *payload.HeadersMessage {
	var hdrs []*payload.BlockBase

	for i := 0; i < num; i++ {
		hash := randomUint256(t)
		hdr := &payload.BlockBase{Hash: hash}
		hdrs = append(hdrs, hdr)
	}

	hdrsMsg, err := payload.NewHeadersMessage()
	assert.Nil(t, err)

	hdrsMsg.Headers = hdrs

	return hdrsMsg
}

func randomUint256(t *testing.T) util.Uint256 {
	hash := make([]byte, 32)
	_, err := rand.Read(hash)
	assert.Nil(t, err)

	u, err := util.Uint256DecodeBytes(hash)
	assert.Nil(t, err)
	return u
}
