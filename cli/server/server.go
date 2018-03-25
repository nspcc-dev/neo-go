package server

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/network"
	"github.com/CityOfZion/neo-go/pkg/rpc"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// NewCommand creates a new Node command.
func NewCommand() cli.Command {
	return cli.Command{
		Name:   "node",
		Usage:  "start a NEO node",
		Action: startServer,
		Flags: []cli.Flag{
			cli.StringFlag{Name: "config-path"},
			cli.BoolFlag{Name: "privnet, p"},
			cli.BoolFlag{Name: "mainnet, m"},
			cli.BoolFlag{Name: "testnet, t"},
			cli.BoolFlag{Name: "debug, d"},
		},
	}
}

func startServer(ctx *cli.Context) error {
	net := config.ModePrivNet
	if ctx.Bool("testnet") {
		net = config.ModeTestNet
	}
	if ctx.Bool("mainnet") {
		net = config.ModeMainNet
	}

	configPath := "./config"
	configPath = ctx.String("config-path")
	cfg, err := config.Load(configPath, net)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt)

	serverConfig := network.NewServerConfig(cfg)
	chain, err := newBlockchain(cfg)
	if err != nil {
		err = fmt.Errorf("could not initialize blockchain: %s", err)
		return cli.NewExitError(err, 1)
	}

	if ctx.Bool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	server := network.NewServer(serverConfig, chain)
	rpcServer := rpc.NewServer(chain, cfg.ApplicationConfiguration.RPCPort, server)
	errChan := make(chan error)

	go server.Start(errChan)
	go rpcServer.Start(errChan)

	fmt.Println(logo())
	fmt.Println(server.UserAgent)
	fmt.Println()

	var shutdownErr error
Main:
	for {
		select {
		case err := <-errChan:
			shutdownErr = errors.Wrap(err, "Error encountered by server")
			interruptChan <- os.Kill

		case <-interruptChan:
			server.Shutdown()
			if serverErr := rpcServer.Shutdown(); serverErr != nil {
				shutdownErr = errors.Wrap(serverErr, "Error encountered whilst shutting down server")
			}
			break Main
		}
	}

	if shutdownErr != nil {
		return cli.NewExitError(shutdownErr, 1)
	}

	return nil
}

func newBlockchain(cfg config.Config) (*core.Blockchain, error) {
	// Hardcoded for now.
	store, err := storage.NewLevelDBStore(
		cfg.ApplicationConfiguration.DataDirectoryPath,
		nil,
	)
	if err != nil {
		return nil, err
	}

	return core.NewBlockchain(store, cfg.ProtocolConfiguration)
}

func logo() string {
	return `
    _   ____________        __________
   / | / / ____/ __ \      / ____/ __ \
  /  |/ / __/ / / / /_____/ / __/ / / /
 / /|  / /___/ /_/ /_____/ /_/ / /_/ /
/_/ |_/_____/\____/      \____/\____/
`
}
