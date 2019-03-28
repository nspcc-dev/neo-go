package syncmgr

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/chain"

	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/stretchr/testify/assert"
)

func TestHeadersModeOnBlock(t *testing.T) {

	syncmgr, helper := setupSyncMgr(headersMode)

	syncmgr.OnBlock(&mockPeer{}, randomBlockMessage(t, 0))

	// In headerMode, we do nothing
	assert.Equal(t, 0, helper.blocksProcessed)
}

func TestBlockModeOnBlock(t *testing.T) {

	syncmgr, helper := setupSyncMgr(blockMode)

	syncmgr.OnBlock(&mockPeer{}, randomBlockMessage(t, 0))

	// When a block is received in blockMode, it is processed
	assert.Equal(t, 1, helper.blocksProcessed)
}
func TestNormalModeOnBlock(t *testing.T) {

	syncmgr, helper := setupSyncMgr(normalMode)

	syncmgr.OnBlock(&mockPeer{}, randomBlockMessage(t, 0))

	// When a block is received in normal, it is processed
	assert.Equal(t, 1, helper.blocksProcessed)
}

func TestBlockModeToNormalMode(t *testing.T) {

	syncmgr, _ := setupSyncMgr(blockMode)

	peer := &mockPeer{
		height: 100,
	}

	blkMessage := randomBlockMessage(t, 100)

	syncmgr.OnBlock(peer, blkMessage)

	// We should switch to normal mode, since the block
	//we received is close to the height of the peer. See cruiseHeight
	assert.Equal(t, normalMode, syncmgr.syncmode)

}
func TestBlockModeStayInBlockMode(t *testing.T) {

	syncmgr, _ := setupSyncMgr(blockMode)

	// We need our latest know hash to not be equal to the hash
	// of the block we received, to stay in blockmode
	syncmgr.headerHash = randomUint256(t)

	peer := &mockPeer{
		height: 2000,
	}

	blkMessage := randomBlockMessage(t, 100)

	syncmgr.OnBlock(peer, blkMessage)

	// We should stay in block mode, since the block we received is
	// still quite far behind the peers height
	assert.Equal(t, blockMode, syncmgr.syncmode)
}
func TestBlockModeAlreadyExistsErr(t *testing.T) {

	syncmgr, helper := setupSyncMgr(blockMode)
	helper.err = chain.ErrBlockAlreadyExists

	syncmgr.OnBlock(&mockPeer{}, randomBlockMessage(t, 100))

	assert.Equal(t, 0, helper.blockFetchRequest)

	// If we have a block already exists in blockmode, then we
	// switch back to headers mode.
	assert.Equal(t, headersMode, syncmgr.syncmode)
}

func randomBlockMessage(t *testing.T, height uint32) *payload.BlockMessage {
	blockMessage, err := payload.NewBlockMessage()
	blockMessage.BlockBase.Index = height
	assert.Nil(t, err)
	return blockMessage
}
