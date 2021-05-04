package manifest

// This file contains types and helper methods for wildcard containers.
// Wildcard container can contain either a finite set of elements or
// every possible element, in which case it is named `wildcard`.

import (
	"bytes"
	"encoding/json"
)

// WildStrings represents string set which can be wildcard.
type WildStrings struct {
	Value []string
}

// WildPermissionDescs represents PermissionDescriptor set which can be wildcard.
type WildPermissionDescs struct {
	Value []PermissionDesc
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
func (c *WildPermissionDescs) Contains(v PermissionDesc) bool {
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
func (c *WildPermissionDescs) IsWildcard() bool { return c.Value == nil }

// Restrict transforms container into an empty one.
func (c *WildStrings) Restrict() { c.Value = []string{} }

// Restrict transforms container into an empty one.
func (c *WildPermissionDescs) Restrict() { c.Value = []PermissionDesc{} }

// Add adds v to the container.
func (c *WildStrings) Add(v string) { c.Value = append(c.Value, v) }

// Add adds v to the container.
func (c *WildPermissionDescs) Add(v PermissionDesc) { c.Value = append(c.Value, v) }

// MarshalJSON implements json.Marshaler interface.
func (c WildStrings) MarshalJSON() ([]byte, error) {
	if c.IsWildcard() {
		return []byte(`"*"`), nil
	}
	return json.Marshal(c.Value)
}

// MarshalJSON implements json.Marshaler interface.
func (c WildPermissionDescs) MarshalJSON() ([]byte, error) {
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
func (c *WildPermissionDescs) UnmarshalJSON(data []byte) error {
	if !bytes.Equal(data, []byte(`"*"`)) {
		us := []PermissionDesc{}
		if err := json.Unmarshal(data, &us); err != nil {
			return err
		}
		c.Value = us
	}
	return nil
}
