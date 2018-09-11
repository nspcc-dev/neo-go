package connmgr_test

import (
	"testing"
	"time"

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

	time.Sleep(1 * time.Second) // to not use this, we would need to change the API. To me, it is not worth it, just for test and the 2sec sleep is good enough
	assert.Equal(t, 1, len(cm.ConnectedList))

}
