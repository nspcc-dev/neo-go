package config

// BasicService is used as a simple base for node services like Pprof, RPC or
// Prometheus monitoring.
type BasicService struct {
	Enabled bool `yaml:"Enabled"`
	// Addresses holds the list of bind addresses in the form of "address:port".
	Addresses []string `yaml:"Addresses"`
}
