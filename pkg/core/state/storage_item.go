package state

// StorageItem is the value to be stored with read-only flag.
type StorageItem []byte

// StorageItemWithKey is a storage item with corresponding key.
type StorageItemWithKey struct {
	Key  []byte
	Item StorageItem
}
