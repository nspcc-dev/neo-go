package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBasicService_FormatAddress(t *testing.T) {
	for expected, tc := range map[string]BasicService{
		"localhost:10332": {Address: "localhost", Port: 10332},
		"127.0.0.1:0":     {Address: "127.0.0.1"},
		":0":              {},
	} {
		t.Run(expected, func(t *testing.T) {
			require.Equal(t, expected, tc.FormatAddress())
		})
	}
}
