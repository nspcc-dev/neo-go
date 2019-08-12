package server

import (
	"github.com/CityOfZion/neo-go/pkg/chain"
	"github.com/CityOfZion/neo-go/pkg/database"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
)

func setupChain(db database.Database, net protocol.Magic) (*chain.Chain, error) {
	chain, err := chain.New(db, net)
	if err != nil {
		return nil, err
	}
	return chain, nil
}
