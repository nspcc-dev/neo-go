package config

import (
	"fmt"
	"os"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/rpc"
	"gopkg.in/yaml.v2"
)

const (
	// UserAgentWrapper is a string that user agent string should be wrapped into.
	UserAgentWrapper = "/"
	// UserAgentPrefix is a prefix used to generate user agent string.
	UserAgentPrefix = "NEO-GO:"
	// UserAgentFormat is a formatted string used to generate user agent string.
	UserAgentFormat = UserAgentWrapper + UserAgentPrefix + "%s" + UserAgentWrapper
)

// Version is the version of the node, set at the build time.
var Version string

// Config top level struct representing the config
// for the node.
type Config struct {
	ProtocolConfiguration    ProtocolConfiguration    `yaml:"ProtocolConfiguration"`
	ApplicationConfiguration ApplicationConfiguration `yaml:"ApplicationConfiguration"`
}

// GenerateUserAgent creates a user agent string based on the build time environment.
func (c Config) GenerateUserAgent() string {
	return fmt.Sprintf(UserAgentFormat, Version)
}

// Load attempts to load the config from the given
// path for the given netMode.
func Load(path string, netMode netmode.Magic) (Config, error) {
	configPath := fmt.Sprintf("%s/protocol.%s.yml", path, netMode)
	return LoadFile(configPath)
}

// LoadFile loads config from the provided path.
func LoadFile(configPath string) (Config, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return Config{}, fmt.Errorf("config '%s' doesn't exist", configPath)
	}

	configData, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("unable to read config: %w", err)
	}

	config := Config{
		ApplicationConfiguration: ApplicationConfiguration{
			PingInterval: 30,
			PingTimeout:  90,
			RPC: rpc.Config{
				MaxIteratorResultItems: 100,
				MaxFindResultItems:     100,
				MaxNEP11Tokens:         100,
			},
		},
	}

	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal config YAML: %w", err)
	}

	err = config.ProtocolConfiguration.Validate()
	if err != nil {
		return Config{}, err
	}

	return config, nil
}
