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
