package config

import (
	"fmt"
)

// Logger contains node logger configuration.
type Logger struct {
	LogLevel    string `yaml:"LogLevel"`
	LogPath     string `yaml:"LogPath"`
	LogEncoding string `yaml:"LogEncoding"`
}

// Validate returns an error if Logger configuration is not valid.
func (l Logger) Validate() error {
	if len(l.LogEncoding) > 0 && l.LogEncoding != "console" && l.LogEncoding != "json" {
		return fmt.Errorf("invalid logger encoding: %s", l.LogEncoding)
	}
	return nil
}
