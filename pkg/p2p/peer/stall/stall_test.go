package stall

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
)

func TestAddRemoveMessage(t *testing.T) {
	d := NewDetector(responseTime, tickerInterval)
	d.AddMessage(command.GetAddr)
	mp := d.GetMessages()

	assert.Equal(t, 1, len(mp))
	assert.NotEmpty(t, mp[command.GetAddr])

	d.RemoveMessage(command.GetAddr)
	mp = d.GetMessages()

	assert.Equal(t, 0, len(mp))
	assert.Empty(t, mp[command.GetAddr])
}

func TestDeadline(t *testing.T) {
	responseTime := 2 * time.Second
	tickerInterval := 1 * time.Second
	d := NewDetector(responseTime, tickerInterval)
	d.AddMessage(command.GetAddr)
	time.Sleep(responseTime + 1*time.Second)

	k := make(map[command.Type]time.Time)
	assert.Equal(t, k, d.responses)
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
