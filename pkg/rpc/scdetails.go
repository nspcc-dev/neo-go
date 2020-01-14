package rpc

import "github.com/CityOfZion/neo-go/pkg/rpc/request"

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
	ReturnType           request.StackParamType
	Parameters           []request.StackParamType
}
