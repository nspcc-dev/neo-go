/*
Package dboper contains a type used to represent single DB operation.
*/
package dboper

// Operation represents a single KV operation (add/del/change) performed
// in the DB.
type Operation struct {
	// State can be Added, Changed or Deleted.
	State string `json:"state"`
	Key   []byte `json:"key"`
	Value []byte `json:"value,omitempty"`
}
