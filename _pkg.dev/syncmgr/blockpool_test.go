package syncmgr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddBlockPoolFlush(t *testing.T) {
	syncmgr, _ := setupSyncMgr(blockMode, 10)

	blockMessage := randomBlockMessage(t, 11)

	peer := &mockPeer{
		height: 100,
	}

	// Since the block has Index 11 and the sync manager needs the block with index 10
	// This block will be added to the blockPool
	err := syncmgr.OnBlock(peer, blockMessage)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(syncmgr.blockPool))

	// The sync manager is still looking for the block at height 10
	// Since this block is at height 12, it will be added to the block pool
	blockMessage = randomBlockMessage(t, 12)
	err = syncmgr.OnBlock(peer, blockMessage)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(syncmgr.blockPool))

	// This is the block that the sync manager was waiting for
	// It should process this block, the check the pool for the next set of blocks
	blockMessage = randomBlockMessage(t, 10)
	err = syncmgr.OnBlock(peer, blockMessage)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(syncmgr.blockPool))

	// Since we processed 3 blocks and the sync manager started
	//looking for block with index 10. The syncmananger should be looking for
	// the block with index 13
	assert.Equal(t, uint32(13), syncmgr.nextBlockIndex)
}
