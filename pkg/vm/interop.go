package vm

import "fmt"

// InteropFunc function signature.
type InteropFunc func(vm *VM) error

// InteropService
type InteropService struct {
	mapping map[string]InteropFunc
}

func NewInteropService() *InteropService {
	return &InteropService{
		mapping: map[string]InteropFunc{},
	}
}

// Register any API to the interop service.
func (i *InteropService) Register(api string, fun InteropFunc) {
	i.mapping[api] = fun
}

// Call will invoke the service mapped to the given api.
func (i *InteropService) Call(api []byte, vm *VM) error {
	fun, ok := i.mapping[string(api)]
	if !ok {
		return fmt.Errorf("api (%s) not in interop mapping", api)
	}
	return fun(vm)
}
