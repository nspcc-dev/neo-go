package network

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
)

const (
	userAgentFormat = "/NEO-GO:%s/"
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
	}

	// ProtocolConfiguration represents the protolcol config.
	ProtocolConfiguration struct {
		Magic                   NetMode
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
		DataDirectoryPath string
		RPCPort           int
		NodePort          int
		Relay             bool
	}
)

// GenerateUserAgent creates user agent string based on build time environment.
func (c Config) GenerateUserAgent() string {
	return fmt.Sprintf(userAgentFormat, Version)
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

	return config, nil
}
