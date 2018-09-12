package connmgr_test

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/connmgr"
	"github.com/stretchr/testify/assert"
)

func TestDial(t *testing.T) {
	cfg := connmgr.Config{nil, nil}

	cm := connmgr.New(cfg)
	cm.Run()

	ipport := "google.com:80" // google unlikely to go offline, a better approach to test Dialing is welcome.

	conn, err := cm.Dial(ipport)
	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, conn)
}
func TestConnect(t *testing.T) {
	cfg := connmgr.Config{nil, nil}

	cm := connmgr.New(cfg)
	cm.Run()

	ipport := "google.com:80"

	r := connmgr.Request{Addr: ipport}

	cm.Connect(&r)

	assert.Equal(t, 1, len(cm.ConnectedList))

}
func TestNewRequest(t *testing.T) {

	address := "google.com:80"

	var getAddr = func() (string, error) {
		return address, nil
	}

	cfg := connmgr.Config{getAddr, nil}

	cm := connmgr.New(cfg)
	cm.Run()

	cm.NewRequest()

	if _, ok := cm.ConnectedList[address]; ok {
		assert.Equal(t, true, ok)
		assert.Equal(t, 1, len(cm.ConnectedList))
		return
	}

	t.Fail()

}
