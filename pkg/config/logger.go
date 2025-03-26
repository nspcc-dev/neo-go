package config

import (
	"fmt"
)

// Logger contains node logger configuration.
type Logger struct {
	LogEncoding  string `yaml:"LogEncoding"`
	LogLevel     string `yaml:"LogLevel"`
	LogPath      string `yaml:"LogPath"`
	LogTimestamp *bool  `yaml:"LogTimestamp,omitempty"`
}

// Validate returns an error if Logger configuration is not valid.
func (l Logger) Validate() error {
	if len(l.LogEncoding) > 0 && l.LogEncoding != "console" && l.LogEncoding != "json" {
		return fmt.Errorf("invalid LogEncoding: %s", l.LogEncoding)
	}
	return nil
}
