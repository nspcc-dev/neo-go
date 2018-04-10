package storage

// Context ..
func Context() interface{} { return 0 }

// Put stores a value in to the storage.
func Put(ctx interface{}, key string, value interface{}) {}

// Get returns the value from the storage.
func Get(ctx interface{}, key string) interface{} { return 0 }

// Delete removes a stored key value pair.
func Delete(ctx interface{}, key string) {}
