package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const testConfigPath = "./testdata/protocol.test.yml"

func TestUnexpectedNativeUpdateHistoryContract(t *testing.T) {
	_, err := LoadFile(testConfigPath)
	require.Error(t, err)
}
