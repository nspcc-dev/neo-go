package manifest

// This file contains types and helper methods for wildcard containers.
// Wildcard container can contain either a finite set of elements or
// every possible element, in which case it is named `wildcard`.

import (
	"bytes"
	"encoding/json"

	"github.com/nspcc-dev/neo-go/pkg/util"
)

// WildStrings represents string set which can be wildcard.
type WildStrings struct {
	Value []string
}

// WildUint160s represents Uint160 set which can be wildcard.
type WildUint160s struct {
	Value []util.Uint160
}

// Contains checks if v is in the container.
func (c *WildStrings) Contains(v string) bool {
	if c.IsWildcard() {
		return true
	}
	for _, s := range c.Value {
		if v == s {
			return true
		}
	}
	return false
}

// Contains checks if v is in the container.
func (c *WildUint160s) Contains(v util.Uint160) bool {
	if c.IsWildcard() {
		return true
	}
	for _, u := range c.Value {
		if u.Equals(v) {
			return true
		}
	}
	return false
}

// IsWildcard returns true iff container is wildcard.
func (c *WildStrings) IsWildcard() bool { return c.Value == nil }

// IsWildcard returns true iff container is wildcard.
func (c *WildUint160s) IsWildcard() bool { return c.Value == nil }

// Restrict transforms container into an empty one.
func (c *WildStrings) Restrict() { c.Value = []string{} }

// Restrict transforms container into an empty one.
func (c *WildUint160s) Restrict() { c.Value = []util.Uint160{} }

// Add adds v to the container.
func (c *WildStrings) Add(v string) { c.Value = append(c.Value, v) }

// Add adds v to the container.
func (c *WildUint160s) Add(v util.Uint160) { c.Value = append(c.Value, v) }

// MarshalJSON implements json.Marshaler interface.
func (c WildStrings) MarshalJSON() ([]byte, error) {
	if c.IsWildcard() {
		return []byte(`"*"`), nil
	}
	return json.Marshal(c.Value)
}

// MarshalJSON implements json.Marshaler interface.
func (c WildUint160s) MarshalJSON() ([]byte, error) {
	if c.IsWildcard() {
		return []byte(`"*"`), nil
	}
	return json.Marshal(c.Value)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (c *WildStrings) UnmarshalJSON(data []byte) error {
	if !bytes.Equal(data, []byte(`"*"`)) {
		ss := []string{}
		if err := json.Unmarshal(data, &ss); err != nil {
			return err
		}
		c.Value = ss
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (c *WildUint160s) UnmarshalJSON(data []byte) error {
	if !bytes.Equal(data, []byte(`"*"`)) {
		us := []util.Uint160{}
		if err := json.Unmarshal(data, &us); err != nil {
			return err
		}
		c.Value = us
	}
	return nil
}
