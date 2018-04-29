package storage

// Context represents the storage context.
type Context interface{}

// GetContext returns the storage context.
func GetContext() interface{} { return nil }

// Put stores a value in to the storage.
func Put(ctx interface{}, key interface{}, value interface{}) {}

// Get returns the value from the storage.
func Get(ctx interface{}, key interface{}) interface{} { return 0 }

// Delete removes a stored key value pair.
func Delete(ctx interface{}, key interface{}) {}
