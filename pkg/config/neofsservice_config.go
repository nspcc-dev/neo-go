package config

import (
	"errors"
	"fmt"
	"time"

	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
)

// NeoFSService represents the configuration for services interacting
// with NeoFS block/state storage.
type NeoFSService struct {
	InternalService `yaml:",inline"`
	Timeout         time.Duration `yaml:"Timeout"`
	ContainerID     string        `yaml:"ContainerID"`
	Addresses       []string      `yaml:"Addresses"`
}

// Validate checks NeoFSService for internal consistency.
func (cfg *NeoFSService) Validate() error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.ContainerID == "" {
		return errors.New("container ID is not set")
	}
	var containerID cid.ID
	err := containerID.DecodeString(cfg.ContainerID)
	if err != nil {
		return fmt.Errorf("invalid container ID: %w", err)
	}
	if len(cfg.Addresses) == 0 {
		return errors.New("addresses are not set")
	}
	return nil
}

// NeoFSBlockFetcher represents the configuration for the NeoFS BlockFetcher service.
type NeoFSBlockFetcher struct {
	NeoFSService           `yaml:",inline"`
	OIDBatchSize           int    `yaml:"OIDBatchSize"`
	BlockAttribute         string `yaml:"BlockAttribute"`
	IndexFileAttribute     string `yaml:"IndexFileAttribute"`
	DownloaderWorkersCount int    `yaml:"DownloaderWorkersCount"`
	BQueueSize             int    `yaml:"BQueueSize"`
	SkipIndexFilesSearch   bool   `yaml:"SkipIndexFilesSearch"`
	IndexFileSize          uint32 `yaml:"IndexFileSize"`
}

// Validate checks NeoFSBlockFetcher for internal consistency and ensures
// that all required fields are properly set. It returns an error if the
// configuration is invalid or if the ContainerID cannot be properly decoded.
func (cfg *NeoFSBlockFetcher) Validate() error {
	if err := cfg.NeoFSService.Validate(); err != nil {
		return err
	}
	if cfg.BQueueSize > 0 && cfg.BQueueSize < cfg.OIDBatchSize {
		return fmt.Errorf("BQueueSize (%d) is lower than OIDBatchSize (%d)", cfg.BQueueSize, cfg.OIDBatchSize)
	}
	return nil
}
