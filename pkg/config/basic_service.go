package config

// BasicService is used for simple services like Pprof or Prometheus monitoring.
type BasicService struct {
	Enabled bool   `yaml:"Enabled"`
	Address string `yaml:"Address"`
	Port    string `yaml:"Port"`
}
