package stall

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
)

func TestAddRemoveMessage(t *testing.T) {

	responseTime := 2 * time.Second
	tickerInterval := 1 * time.Second

	d := NewDetector(responseTime, tickerInterval)
	d.AddMessage(command.GetAddr)
	mp := d.GetMessages()

	assert.Equal(t, 1, len(mp))
	assert.IsType(t, time.Time{}, mp[command.GetAddr])

	d.RemoveMessage(command.GetAddr)
	mp = d.GetMessages()

	assert.Equal(t, 0, len(mp))
	assert.Empty(t, mp[command.GetAddr])
}

type mockPeer struct {
	lock     *sync.RWMutex
	online   bool
	detector *Detector
}

func (mp *mockPeer) loop() {
loop:
	for {
		select {
		case <-mp.detector.Quitch:

			break loop
		}
	}
	// cleanup
	mp.lock.Lock()
	mp.online = false
	mp.lock.Unlock()
}
func TestDeadlineWorks(t *testing.T) {

	responseTime := 2 * time.Second
	tickerInterval := 1 * time.Second

	d := NewDetector(responseTime, tickerInterval)
	mp := mockPeer{online: true, detector: d, lock: new(sync.RWMutex)}
	go mp.loop()

	d.AddMessage(command.GetAddr)
	time.Sleep(responseTime + 1*time.Second)

	k := make(map[command.Type]time.Time)
	d.lock.RLock()
	assert.Equal(t, k, d.responses)
	d.lock.RUnlock()
	mp.lock.RLock()
	assert.Equal(t, false, mp.online)
	mp.lock.RUnlock()
}
func TestDeadlineShouldNotBeEmpty(t *testing.T) {
	responseTime := 10 * time.Second
	tickerInterval := 1 * time.Second

	d := NewDetector(responseTime, tickerInterval)
	d.AddMessage(command.GetAddr)
	time.Sleep(1 * time.Second)

	k := make(map[command.Type]time.Time)
	d.lock.RLock()
	assert.NotEqual(t, k, d.responses)
	d.lock.RUnlock()
}
