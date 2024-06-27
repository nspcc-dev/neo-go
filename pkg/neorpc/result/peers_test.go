package result

import (
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/network"
	"github.com/stretchr/testify/require"
)

func TestGetPeers(t *testing.T) {
	gp := NewGetPeers()
	require.Equal(t, 0, len(gp.Unconnected))
	require.Equal(t, 0, len(gp.Connected))
	require.Equal(t, 0, len(gp.Bad))

	gp.AddUnconnected([]string{"1.1.1.1:53", "8.8.8.8:53", "9.9.9.9:53"})
	unsupportedFormat := "2001:DB0:0:123A:::30"
	gp.AddConnected([]network.PeerInfo{
		{Address: "192.168.0.1:10333", UserAgent: "/NEO-GO:0.106.2/", Height: 100},
		{Address: unsupportedFormat, UserAgent: "", Height: 0},
		{Address: "[2001:DB0:0:123A::]:30", UserAgent: "/NEO-GO:0.106.2/", Height: 200},
	})
	gp.AddBad([]string{"127.0.0.1:20333", "127.0.0.1:65536"})

	require.Equal(t, 3, len(gp.Unconnected))
	require.Equal(t, 2, len(gp.Connected))
	require.Equal(t, 2, len(gp.Bad))
	require.Equal(t, "192.168.0.1", gp.Connected[0].Address)
	require.Equal(t, uint16(10333), gp.Connected[0].Port)
	require.Equal(t, "/NEO-GO:0.106.2/", gp.Connected[0].UserAgent)
	require.Equal(t, uint32(100), gp.Connected[0].LastKnownHeight)
	require.Equal(t, uint16(30), gp.Connected[1].Port)
	require.Equal(t, "/NEO-GO:0.106.2/", gp.Connected[1].UserAgent)
	require.Equal(t, uint32(200), gp.Connected[1].LastKnownHeight)
	require.Equal(t, "127.0.0.1", gp.Bad[0].Address)
	require.Equal(t, uint16(20333), gp.Bad[0].Port)

	gps := GetPeers{}
	oldPeerFormat := `{"unconnected": [{"address": "20.109.188.128","port": "10333"},{"address": "27.188.182.47","port": "10333"}],"connected": [{"address": "54.227.43.72","port": "10333"},{"address": "157.90.177.38","port": "10333"}],"bad": [{"address": "5.226.142.226","port": "10333"}]}`
	err := json.Unmarshal([]byte(oldPeerFormat), &gps)
	require.NoError(t, err)
	newPeerFormat := `{"unconnected": [{"address": "20.109.188.128","port": 10333},{"address": "27.188.182.47","port": 10333}],"connected": [{"address": "54.227.43.72","port": 10333},{"address": "157.90.177.38","port": 10333}],"bad": [{"address": "5.226.142.226","port": 10333},{"address": "54.208.117.178","port": 10333}]}`
	err = json.Unmarshal([]byte(newPeerFormat), &gps)
	require.NoError(t, err)
	badIntFormat := `{"unconnected": [{"address": "20.109.188.128","port": 65536}],"connected": [],"bad": []}`
	err = json.Unmarshal([]byte(badIntFormat), &gps)
	require.Error(t, err)
	badStringFormat := `{"unconnected": [{"address": "20.109.188.128","port": "badport"}],"connected": [],"bad": []}`
	err = json.Unmarshal([]byte(badStringFormat), &gps)
	require.Error(t, err)
}
