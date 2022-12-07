package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBasicService_GetAddresses(t *testing.T) {
	addr := "1.2.3.4"
	port := uint16(1234)
	s := BasicService{
		Enabled: false,
		Address: &addr,
		Port:    &port,
		Addresses: []string{"1.2.3.4:1234", /* same as Address:Port */
			"3.4.5.6:1234", "2.3.4.5", ":1235", "2.3.4.5:1234",
			"3.4.5.6:1234" /* already in list */},
	}
	require.Equal(t, []string{
		"1.2.3.4:1234",
		"3.4.5.6:1234",
		"2.3.4.5",
		":1235",
		"2.3.4.5:1234",
		"3.4.5.6:1234",
		"1.2.3.4:1234",
	}, s.GetAddresses())
}
