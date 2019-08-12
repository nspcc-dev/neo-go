package server

import (
	"github.com/CityOfZion/neo-go/pkg/database"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
)

func setupDatabase(net protocol.Magic) (database.Database, error) {
	db, err := database.New(net.String())
	if err != nil {
		return nil, err
	}
	return db, nil
}
