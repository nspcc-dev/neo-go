package syncmanager

import (
	"github.com/CityOfZion/neo-go/pkg/blockchain"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type Config struct {
	Chain    *blockchain.Chain
	BestHash util.Uint256
}
