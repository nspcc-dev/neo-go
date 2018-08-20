package storage

// Context represents the storage context
type Context interface{}

// GetContext returns the storage context
func GetContext() interface{} { return nil }

// Put value at given key
func Put(ctx interface{}, key interface{}, value interface{}) {}

// Get value matching given key
func Get(ctx interface{}, key interface{}) interface{} { return 0 }

// Delete key value pair from storage
func Delete(ctx interface{}, key interface{}) {}

// Find values stored on keys partially matching given key
func Find(ctx interface{}, key interface{}) interface{} { return 0 }
