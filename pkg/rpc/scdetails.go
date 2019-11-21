package rpc

// ContractDetails contains contract metadata.
type ContractDetails struct {
	Author               string
	Email                string
	Version              string
	ProjectName          string `yaml:"name"`
	Description          string
	HasStorage           bool
	HasDynamicInvocation bool
	IsPayable            bool
	ReturnType           StackParamType
	Parameters           []StackParamType
}
