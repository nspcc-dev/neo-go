package connmgr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDial(t *testing.T) {
	cfg := Config{
		GetAddress:   nil,
		OnConnection: nil,
		OnAccept:     nil,
		AddressPort:  "",
		DialTimeout:  0,
	}

	cm := New(cfg)
	err := cm.Run()
	assert.Equal(t, nil, err)

	ipport := "google.com:80" // google unlikely to go offline, a better approach to test Dialing is welcome.

	conn, err := cm.dial(ipport)
	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, conn)
}
func TestConnect(t *testing.T) {
	cfg := Config{
		GetAddress:   nil,
		OnConnection: nil,
		OnAccept:     nil,
		AddressPort:  "",
		DialTimeout:  0,
	}

	cm := New(cfg)
	cm.Run()

	ipport := "google.com:80"

	r := Request{Addr: ipport}

	err := cm.Connect(&r)
	assert.Nil(t, err)

	assert.Equal(t, 1, len(cm.ConnectedList))

}
func TestNewRequest(t *testing.T) {

	address := "google.com:80"

	var getAddr = func() (string, error) {
		return address, nil
	}

	cfg := Config{
		GetAddress:   getAddr,
		OnConnection: nil,
		OnAccept:     nil,
		AddressPort:  "",
		DialTimeout:  0,
	}

	cm := New(cfg)

	cm.Run()

	cm.NewRequest()

	if _, ok := cm.ConnectedList[address]; ok {
		assert.Equal(t, true, ok)
		assert.Equal(t, 1, len(cm.ConnectedList))
		return
	}

	assert.Fail(t, "Could not find the address in the connected lists")

}
func TestDisconnect(t *testing.T) {

	address := "google.com:80"

	var getAddr = func() (string, error) {
		return address, nil
	}

	cfg := Config{
		GetAddress:   getAddr,
		OnConnection: nil,
		OnAccept:     nil,
		AddressPort:  "",
		DialTimeout:  0,
	}

	cm := New(cfg)

	cm.Run()

	cm.NewRequest()

	cm.Disconnect(address)

	assert.Equal(t, 0, len(cm.ConnectedList))

}
