package syncmgr

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/chain"

	"github.com/stretchr/testify/assert"

	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

func TestHeadersModeOnHeaders(t *testing.T) {

	syncmgr, helper := setupSyncMgr(headersMode)

	syncmgr.OnHeader(&mockPeer{}, randomHeadersMessage(t, 0))

	// Since there were no headers, we should have exited early and processed nothing
	assert.Equal(t, 0, helper.headersProcessed)

	// ProcessHeaders should have been called once to process all 100 headers
	syncmgr.OnHeader(&mockPeer{}, randomHeadersMessage(t, 100))
	assert.Equal(t, 100, helper.headersProcessed)

	// Mode should now be blockMode
	assert.Equal(t, blockMode, syncmgr.syncmode)

}

func TestBlockModeOnHeaders(t *testing.T) {
	syncmgr, helper := setupSyncMgr(blockMode)

	// If we receive a header in blockmode, no headers will be processed
	syncmgr.OnHeader(&mockPeer{}, randomHeadersMessage(t, 100))
	assert.Equal(t, 0, helper.headersProcessed)
}
func TestNormalModeOnHeadersMaxHeaders(t *testing.T) {
	syncmgr, helper := setupSyncMgr(normalMode)

	// If we receive a header in normalmode, headers will be processed
	syncmgr.OnHeader(&mockPeer{}, randomHeadersMessage(t, 2000))
	assert.Equal(t, 2000, helper.headersProcessed)

	// Mode should now be headersMode since we received 2000 headers
	assert.Equal(t, headersMode, syncmgr.syncmode)
}

// This differs from the previous function in that
//we did not receive the max amount of headers
func TestNormalModeOnHeaders(t *testing.T) {
	syncmgr, helper := setupSyncMgr(normalMode)

	// If we receive a header in normalmode, headers will be processed
	syncmgr.OnHeader(&mockPeer{}, randomHeadersMessage(t, 200))
	assert.Equal(t, 200, helper.headersProcessed)

	// Because we did not receive 2000 headers, we switch to blockMode
	assert.Equal(t, blockMode, syncmgr.syncmode)
}

func TestLastHeaderUpdates(t *testing.T) {
	syncmgr, _ := setupSyncMgr(headersMode)

	hdrsMessage := randomHeadersMessage(t, 200)
	hdrs := hdrsMessage.Headers
	lastHeader := hdrs[len(hdrs)-1]

	syncmgr.OnHeader(&mockPeer{}, hdrsMessage)

	// Headers are processed in headersMode
	// Last header should be updated
	assert.True(t, syncmgr.headerHash.Equals(lastHeader.Hash))

	// Change mode to blockMode and reset lastHeader
	syncmgr.syncmode = blockMode
	syncmgr.headerHash = util.Uint256{}

	syncmgr.OnHeader(&mockPeer{}, hdrsMessage)

	// header should not be changed
	assert.False(t, syncmgr.headerHash.Equals(lastHeader.Hash))

	// Change mode to normalMode and reset lastHeader
	syncmgr.syncmode = normalMode
	syncmgr.headerHash = util.Uint256{}

	syncmgr.OnHeader(&mockPeer{}, hdrsMessage)

	// headers are processed in normalMode
	// hash should be updated
	assert.True(t, syncmgr.headerHash.Equals(lastHeader.Hash))

}

func TestHeadersModeOnHeadersErr(t *testing.T) {

	syncmgr, helper := setupSyncMgr(headersMode)
	helper.err = &chain.ValidationError{}

	syncmgr.OnHeader(&mockPeer{}, randomHeadersMessage(t, 200))

	// On a validation error, we should request for another peer
	// to send us these headers
	assert.Equal(t, 1, helper.headersFetchRequest)
}

func TestNormalModeOnHeadersErr(t *testing.T) {
	syncmgr, helper := setupSyncMgr(normalMode)
	helper.err = &chain.ValidationError{}

	syncmgr.OnHeader(&mockPeer{}, randomHeadersMessage(t, 200))

	// On a validation error, we should request for another peer
	// to send us these headers
	assert.Equal(t, 1, helper.headersFetchRequest)
}
