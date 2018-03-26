package main

import (
	"github.com/CityOfZion/neo-go/pkg/vm"
)

func main() {
	v := vm.New(nil)
	v.Load("../contract.avm")
	v.Run()
}
