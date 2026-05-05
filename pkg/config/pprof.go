package config

// Pprof wraps [BasicService] and adds a number of pprof-specific options.
type Pprof struct {
	BasicService `yaml:",inline"`

	EnableBlock bool `yaml:"EnableBlock"`
	EnableMutex bool `yaml:"EnableMutex"`
}
