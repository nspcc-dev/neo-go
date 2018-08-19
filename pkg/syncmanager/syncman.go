package syncmanager

import (
	"errors"
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/blockchain"
	"github.com/CityOfZion/neo-go/pkg/p2p/peer"
	"github.com/CityOfZion/neo-go/pkg/pubsub"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
)

type Syncmanager struct {
	Mode    int // 1 = headersFirst, 2 = Blocks, 3 = Maintain
	chain   *blockchain.Chain
	headers []*payload.BlockBase
}

func New(chain *blockchain.Chain) *Syncmanager {
	return &Syncmanager{
		1,
		chain,
		nil,
	}
}

func (s *Syncmanager) OnHeaders(p *peer.Peer, msg *payload.HeadersMessage) {
	fmt.Println("Sync manager On Headers called")
	// On receipt of Headers
	// check what mode we are in
	// HeadersMode, we check if there is 2k. If so call again. If not then change mode into BlocksOnly

	if s.Mode == 1 {
		err := s.HeadersFirstMode(p, msg)
		if err != nil {
			fmt.Println("Error re blocks", err)
			return // We should custom name error so, that we can do something on WrongHash Error, Peer disconnect error
		}
		return

	}

	//If we receive a Header while in BlocksOnly mode
	// We just save the headers.
	if s.Mode == 2 {
		// Just save header while in BlocksOnly Mode
		err := s.addHeaders(msg)
		if err != nil {
			p.Disconnect()
			fmt.Println("Bad Header, disconnecting Peer")
		}
	}

	// Maintain: We may receive headers when in maintain mode
	// We save it and ask for the block that goes with it
	if s.Mode == 3 {

	}

}

func (s *Syncmanager) HeadersFirstMode(p *peer.Peer, msg *payload.HeadersMessage) error {

	// We add headers in here instead of the blockchain
	// because the syncmanager will need to manage inflight blocks,
	// and re-requesting headers that have failed.
	err := s.addHeaders(msg)
	if err != nil {
		p.Disconnect()
		fmt.Println("Error Adding Headers, Disconnecting peer")
		return err
	}
	if len(msg.Headers) == 2000 { // should be less than 200, leave it as this for tests
		fmt.Println("Switching to BlocksOnly Mode")
		s.Mode = 2 // switch to BlocksOnly

		return p.RequestBlocks(s.headers) // We can do this because headers will only hold the freshly gotten headers, when in HeadersFirstMode
	}
	lastHeader := msg.Headers[len(msg.Headers)-1]
	err = p.RequestHeaders(lastHeader.Hash)
	return err
}

//addHeaders is not safe for concurrent access
func (s *Syncmanager) addHeaders(msg *payload.HeadersMessage) error {
	fmt.Println("Adding Headers into List")
	// iterate headers
	for _, currentHeader := range msg.Headers {

		if len(s.headers) == 0 { // Add the genesis hash on blockchain init, for now just check for nil and add
			s.headers = append(s.headers, currentHeader)
			continue
		}
		// Check if header links and add to list, if not then return an error
		lastHeader := s.headers[len(s.headers)-1]
		lastHeaderHash := lastHeader.Hash

		if currentHeader.PrevHash != lastHeaderHash {
			return errors.New("Last Header hash != current header hash")
		}
		s.headers = append(s.headers, currentHeader)
	}
	return nil
}

// OnBlock receives a block from a peer, then passes it to the blockchain to process.
// For now, this will be blocking. When made async, ensure that if a bad block
// Is received from a peer, we disconnect the peer, remove all blocks in flight from peer
// and re-request those blocks. Any blocks that are now relying on those re-requested blocks will now need to wait
func (s *Syncmanager) OnBlock(p *peer.Peer, msg *payload.BlockMessage) {
	// We should not deal with chain state in here
	// If a block fails, we will be notified by the pubsub system, to request it again.
	err := s.chain.AddBlock(msg)
	if err != nil {
		// Put headers back in front of queue to fetch block for.
		fmt.Println("Block had an error", err.Error())
	}
}

// Topics will implement the subscriber interface
// for pub-sub. Return type is a list of Topics that
// The syncmanager is interested in receiving information
//about. Can be static or dynamic by making it inside of the syncmanager struct
func (s *Syncmanager) Topics() []pubsub.EventType {
	return []pubsub.EventType{}
}

// Event will
func (s *Syncmanager) Emit(e pubsub.Event) {
	if e.Type == pubsub.NewBlock {
		// do something we have a new block
	}
	// do something with received event
}
