package config

import (
	"errors"
	"fmt"
	"time"

	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
)

// NeoFSBlockFetcher represents the configuration for the NeoFS BlockFetcher service.
type NeoFSBlockFetcher struct {
	InternalService        `yaml:",inline"`
	Timeout                time.Duration `yaml:"Timeout"`
	ContainerID            string        `yaml:"ContainerID"`
	Addresses              []string      `yaml:"Addresses"`
	OIDBatchSize           int           `yaml:"OIDBatchSize"`
	BlockAttribute         string        `yaml:"BlockAttribute"`
	IndexFileAttribute     string        `yaml:"IndexFileAttribute"`
	DownloaderWorkersCount int           `yaml:"DownloaderWorkersCount"`
	BQueueSize             int           `yaml:"BQueueSize"`
	SkipIndexFilesSearch   bool          `yaml:"SkipIndexFilesSearch"`
	IndexFileSize          uint32        `yaml:"IndexFileSize"`
}

// Validate checks NeoFSBlockFetcher for internal consistency and ensures
// that all required fields are properly set. It returns an error if the
// configuration is invalid or if the ContainerID cannot be properly decoded.
func (cfg *NeoFSBlockFetcher) Validate() error {
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
	if cfg.BQueueSize < cfg.OIDBatchSize {
		return fmt.Errorf("BQueueSize (%d) is lower than OIDBatchSize (%d)", cfg.BQueueSize, cfg.OIDBatchSize)
	}
	if len(cfg.Addresses) == 0 {
		return errors.New("addresses are not set")
	}
	if !cfg.SkipIndexFilesSearch && cfg.IndexFileSize == 0 {
		return errors.New("IndexFileSize is not set")
	}
	return nil
}
