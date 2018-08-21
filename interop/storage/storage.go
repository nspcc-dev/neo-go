package storage

// Context represents the storage context
type Context struct{}

// GetContext returns the storage context
func GetContext() Context { return Context{} }

// Put value at given key
func Put(ctx Context, key interface{}, value interface{}) {}

// Get value matching given key
func Get(ctx Context, key interface{}) interface{} { return 0 }

// Delete key value pair from storage
func Delete(ctx Context, key interface{}) {}

// Find values stored on keys partially matching given key
func Find(ctx Context, key interface{}) interface{} { return 0 }
