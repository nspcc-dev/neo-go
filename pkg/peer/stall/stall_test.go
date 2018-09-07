package stall

import (
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
	mp.online = false
}
func TestDeadlineWorks(t *testing.T) {

	responseTime := 2 * time.Second
	tickerInterval := 1 * time.Second

	d := NewDetector(responseTime, tickerInterval)
	mp := mockPeer{online: true, detector: d}
	go mp.loop()

	d.AddMessage(command.GetAddr)
	time.Sleep(responseTime + 1*time.Second)

	k := make(map[command.Type]time.Time)
	assert.Equal(t, k, d.responses)
	assert.Equal(t, false, mp.online)

}
func TestDeadlineShouldNotBeEmpty(t *testing.T) {
	responseTime := 10 * time.Second
	tickerInterval := 1 * time.Second

	d := NewDetector(responseTime, tickerInterval)
	d.AddMessage(command.GetAddr)
	time.Sleep(1 * time.Second)

	k := make(map[command.Type]time.Time)
	assert.NotEqual(t, k, d.responses)
}
