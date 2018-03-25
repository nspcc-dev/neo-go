package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/go-yaml/yaml"
	"github.com/pkg/errors"
)

const (
	userAgentFormat = "/NEO-GO:%s/"

	// Valid NetMode constants.
	ModeMainNet NetMode = 0x00746e41 // 7630401
	ModeTestNet NetMode = 0x74746e41 // 1953787457
	ModePrivNet NetMode = 56753      // docker privnet
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
		ProtocolConfiguration    ProtocolConfiguration    `yaml:"ProtocolConfiguration"`
		ApplicationConfiguration ApplicationConfiguration `yaml:"ApplicationConfiguration"`
	}

	// ProtocolConfiguration represents the protolcol config.
	ProtocolConfiguration struct {
		Magic                   NetMode   `yaml:"Magic"`
		AddressVersion          int64     `yaml:"AddressVersion"`
		MaxTransactionsPerBlock int64     `yaml:"MaxTransactionsPerBlock"`
		StandbyValidators       []string  `yaml:"StandbyValidators"`
		SeedList                []string  `yaml:"SeedList"`
		SystemFee               SystemFee `yaml:"SystemFee"`
	}

	// SystemFee fees related to system.
	SystemFee struct {
		EnrollmentTransaction int64 `yaml:"EnrollmentTransaction"`
		IssueTransaction      int64 `yaml:"IssueTransaction"`
		PublishTransaction    int64 `yaml:"PublishTransaction"`
		RegisterTransaction   int64 `yaml:"RegisterTransaction"`
	}

	// ApplicationConfiguration config specific to the node.
	ApplicationConfiguration struct {
		DataDirectoryPath string        `yaml:"DataDirectoryPath"`
		RPCPort           uint16        `yaml:"RPCPort"`
		NodePort          uint16        `yaml:"NodePort"`
		Relay             bool          `yaml:"Relay"`
		DialTimeout       time.Duration `yaml:"DialTimeout"`
		ProtoTickInterval time.Duration `yaml:"ProtoTickInterval"`
		MaxPeers          int           `yaml:"MaxPeers"`
	}

	// NetMode describes the mode the blockchain will operate on.
	NetMode uint32
)

// String implements the stringer interface.
func (n NetMode) String() string {
	switch n {
	case ModePrivNet:
		return "privnet"
	case ModeTestNet:
		return "testnet"
	case ModeMainNet:
		return "mainnet"
	default:
		return "net unknown"
	}
}

// GenerateUserAgent creates user agent string based on build time environment.
func (c Config) GenerateUserAgent() string {
	return fmt.Sprintf(userAgentFormat, Version)
}

// Loadattempts to load the config from the give
// path and netMode.
func Load(path string, netMode NetMode) (Config, error) {
	configPath := fmt.Sprintf("%s/protocol.%s.yml", path, netMode)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return Config{}, errors.Wrap(err, "Unable to load config")
	}

	configData, err := ioutil.ReadFile(configPath)
	if err != nil {
		return Config{}, errors.Wrap(err, "Unable to read config")
	}

	config := Config{
		ProtocolConfiguration: ProtocolConfiguration{
			SystemFee: SystemFee{},
		},
		ApplicationConfiguration: ApplicationConfiguration{},
	}

	err = yaml.Unmarshal([]byte(configData), &config)
	if err != nil {
		return Config{}, errors.Wrap(err, "Problem unmarshaling config json data")
	}

	return config, nil
}
