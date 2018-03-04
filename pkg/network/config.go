package network

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
)

var (
	// Version the version of the node, set at build time.
	Version string

	// BuildTime the time and date the current version of the node built,
	// set at build time.
	BuildTime string
)

type (
	// Config top level struct representing the config
	// for the node.
	Config struct {
		ProtocolConfiguration    ProtocolConfiguration
		ApplicationConfiguration ApplicationConfiguration
		NetMode                  NetMode
	}

	// ProtocolConfiguration represents the protolcol config.
	ProtocolConfiguration struct {
		Magic                   int64
		AddressVersion          int64
		MaxTransactionsPerBlock int64
		StandbyValidators       []string
		SeedList                []string
		SystemFee               SystemFee
	}

	// SystemFee fees related to system.
	SystemFee struct {
		EnrollmentTransaction int64
		IssueTransaction      int64
		PublishTransaction    int64
		RegisterTransaction   int64
	}

	// ApplicationConfiguration config specific to the node.
	ApplicationConfiguration struct {
		RPCPort  int
		NodePort int
	}
)

// HasSeeds returns true if the config contains a set of seed
// peers to connect to.
func (c Config) HasSeeds() bool {
	return len(c.ProtocolConfiguration.SeedList) > 0
}

// HasValidNetMode returns true if the given netmode is
// valid or not.
func (c Config) HasValidNetMode() bool {
	return c.NetMode == ModeTestNet || c.NetMode == ModeMainNet || c.NetMode == ModePrivNet
}

// LoadConfig attempts to load the config from the give
// path and netMode.
func LoadConfig(path string, netMode NetMode) (Config, error) {
	configPath := fmt.Sprintf("%s/protocol.%s.json", path, netMode)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return Config{}, errors.Wrap(err, "Unable to load config")
	}

	configData, err := ioutil.ReadFile(configPath)
	if err != nil {
		return Config{}, errors.Wrap(err, "Unable to read config")
	}

	var config Config
	err = json.Unmarshal([]byte(configData), &config)
	if err != nil {
		return Config{}, errors.Wrap(err, "Problem unmarshaling config json data")
	}

	config.NetMode = netMode

	return config, nil
}
