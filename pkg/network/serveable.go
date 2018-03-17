package network

// Servable is an interface for structs capable of
// serving a service of some kind.
type Servable interface {
	Start()
}
