package storage

// GetContext ..
func GetContext() interface{} { return 0 }

// Put stores a value in to the storage.
func Put(ctx interface{}, key interface{}, value interface{}) int { return 0 }

// GetInt returns the value as an integer.
func GetInt(ctx interface{}, key interface{}) int { return 0 }

// GetString returns the value as an string.
func GetString(ctx interface{}, key interface{}) string { return "" }

// Delete removes a stored key value pair.
func Delete(ctx interface{}, key interface{}) int { return 0 }
