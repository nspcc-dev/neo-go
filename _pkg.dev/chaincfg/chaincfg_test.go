package chaincfg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMainnet(t *testing.T) {
	p, err := mainnet()
	assert.Nil(t, err)
	assert.Equal(t, p.GenesisBlock.Hash.ReverseString(), "d42561e3d30e15be6400b6df2f328e02d2bf6354c41dce433bc57687c82144bf")
}
