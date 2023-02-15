package rpcclient

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetEndpoint(t *testing.T) {
	host := "http://localhost:1234"
	u, err := url.Parse(host)
	require.NoError(t, err)
	client := Client{
		endpoint: u,
	}
	require.Equal(t, host, client.Endpoint())
}
