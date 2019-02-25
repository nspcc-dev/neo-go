package stall

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
)

// stall detector will keep track of all pendingMessages
// If any message takes too long to reply
// the detector will disconnect the peer

type Detector struct {
	responseTime time.Duration
	tickInterval time.Duration

	lock      sync.Mutex
	responses map[command.Type]time.Time

	// The detector is embedded into a peer and the peer watches this quit chan
	// If this chan is closed, the peer disconnects
	Quitch chan struct{}

	// atomic vals
	disconnected int32
}

// rT is the responseTime and signals how long
// a peer has to reply back to a sent message
// tickerInterval is how often the detector wil check for stalled messages
func NewDetector(rTime time.Duration, tickerInterval time.Duration) *Detector {
	d := &Detector{
		responseTime: rTime,
		tickInterval: tickerInterval,
		lock:         sync.Mutex{},
		responses:    map[command.Type]time.Time{},
		Quitch:       make(chan struct{}),
	}
	go d.loop()
	return d
}

func (d *Detector) loop() {
	ticker := time.NewTicker(d.tickInterval)

loop:
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			for _, deadline := range d.responses {
				if now.After(deadline) {
					fmt.Println("Deadline passed")
					ticker.Stop()
					break loop
				}
			}

		}
	}
	d.Quit()
	d.DeleteAll()
	ticker.Stop()
}

// Quit is a concurrent safe way to call the Quit channel
// Without blocking
func (d *Detector) Quit() {
	// return if already disconnected
	if atomic.LoadInt32(&d.disconnected) != 0 {
		return
	}

	atomic.AddInt32(&d.disconnected, 1)
	close(d.Quitch)
}

// Call this function when we send a message to a peer
// The command passed through is the command that we sent
// and not the command we expect to receive
func (d *Detector) AddMessage(cmd command.Type) {
	cmds := d.addMessage(cmd)
	d.lock.Lock()
	for _, cmd := range cmds {
		d.responses[cmd] = time.Now().Add(d.responseTime)
	}
	d.lock.Unlock()
}

// Call this function when we receive a message from
// peer. This will remove the pendingresponse message from the map.
// The command passed through is the command we received
func (d *Detector) RemoveMessage(cmd command.Type) {
	cmds := d.addMessage(cmd)
	d.lock.Lock()
	for _, cmd := range cmds {
		delete(d.responses, cmd)
	}
	d.lock.Unlock()
}

// DeleteAll empties the map of all contents and
// is called when the detector is being shut down
func (d *Detector) DeleteAll() {
	d.lock.Lock()
	d.responses = make(map[command.Type]time.Time)
	d.lock.Unlock()
}

// GetMessages Will return a map of all of the pendingResponses
// and their deadlines
func (d *Detector) GetMessages() map[command.Type]time.Time {
	var resp map[command.Type]time.Time
	d.lock.Lock()
	resp = d.responses
	d.lock.Unlock()
	return resp
}

// when a message is added, we will add a deadline for
// expected response
func (d *Detector) addMessage(cmd command.Type) []command.Type {

	cmds := []command.Type{}

	switch cmd {
	case command.GetHeaders:
		// We now will expect a Headers Message
		cmds = append(cmds, command.Headers)
	case command.GetAddr:
		// We now will expect a Headers Message
		cmds = append(cmds, command.Addr)

	case command.GetData:
		// We will now expect a block/tx message
		// We can optimise this by including the exact inventory type, however it is not needed
		cmds = append(cmds, command.Block)
		cmds = append(cmds, command.TX)
	case command.GetBlocks:
		// we will now expect a inv message
		cmds = append(cmds, command.Inv)
	case command.Version:
		// We will now expect a verack
		cmds = append(cmds, command.Verack)
	}
	return cmds
}

// if receive a message, we will delete it from pending
func (d *Detector) removeMessage(cmd command.Type) []command.Type {

	cmds := []command.Type{}

	switch cmd {
	case command.Block:
		// We will now expect a block/tx message
		cmds = append(cmds, command.Block)
		cmds = append(cmds, command.TX)
	case command.TX:
		// We will now expect a block/tx message
		cmds = append(cmds, command.Block)
		cmds = append(cmds, command.TX)
	case command.GetBlocks:
		// we will now expect a inv message
		cmds = append(cmds, command.Inv)
	default:
		// We will now expect a verack
		cmds = append(cmds, cmd)
	}
	return cmds
}
