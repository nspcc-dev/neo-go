package stall

import (
	"sync"
	"time"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
)

// stall detector will keep track of all pendingMessages
// If any message takes too long to reply
// the detector will disconnect the peer

const (
	// nodes will have `responseTime` seconds to reply with a response
	responseTime = 60 * time.Second

	tickerInterval = 10 * time.Second
)

type Detector struct {
	responseTime time.Duration
	tickInterval time.Duration

	lock      sync.Mutex
	responses map[command.Type]time.Time

	shutdownPeer func()
}

func NewDetector(deadline time.Duration, tickerInterval time.Duration) *Detector {
	d := &Detector{
		responseTime: deadline,
		tickInterval: tickerInterval,
		lock:         sync.Mutex{},
		responses:    map[command.Type]time.Time{},
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
					break loop
				}
			}

		}
	}
	d.shutdownPeer()
	d.DeleteAll()
	ticker.Stop()

	// close the peer connection

}

func (d *Detector) AddMessage(cmd command.Type) {
	cmds := d.addMessage(cmd)
	d.lock.Lock()
	for _, cmd := range cmds {
		d.responses[cmd] = time.Now().Add(d.responseTime)
	}
	d.lock.Unlock()
}
func (d *Detector) RemoveMessage(cmd command.Type) {
	d.lock.Lock()
	delete(d.responses, cmd)
	d.lock.Unlock()
}
func (d *Detector) DeleteAll() {
	d.lock.Lock()
	d.responses = make(map[command.Type]time.Time)
	d.lock.Unlock()
}

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

	case command.GetData:
		// We will now expect a block/tx message
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
