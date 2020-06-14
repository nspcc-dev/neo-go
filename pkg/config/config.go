package config

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/go-yaml/yaml"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/pkg/errors"
)

const userAgentFormat = "/NEO-GO:%s/"

// Version the version of the node, set at build time.
var Version string

// Config top level struct representing the config
// for the node.
type Config struct {
	ProtocolConfiguration    ProtocolConfiguration    `yaml:"ProtocolConfiguration"`
	ApplicationConfiguration ApplicationConfiguration `yaml:"ApplicationConfiguration"`
}

// GenerateUserAgent creates user agent string based on build time environment.
func (c Config) GenerateUserAgent() string {
	return fmt.Sprintf(userAgentFormat, Version)
}

// Load attempts to load the config from the given
// path for the given netMode.
func Load(path string, netMode netmode.Magic) (Config, error) {
	configPath := fmt.Sprintf("%s/protocol.%s.yml", path, netMode)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return Config{}, errors.Wrap(err, "Unable to load config")
	}

	configData, err := ioutil.ReadFile(configPath)
	if err != nil {
		return Config{}, errors.Wrap(err, "Unable to read config")
	}

	config := Config{
		ApplicationConfiguration: ApplicationConfiguration{
			PingInterval: 30,
			PingTimeout:  90,
		},
	}

	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		return Config{}, errors.Wrap(err, "Problem unmarshaling config json data")
	}

	return config, nil
}
