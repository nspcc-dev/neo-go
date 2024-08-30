package manifest

// This file contains types and helper methods for wildcard containers.
// A wildcard container can contain either a finite set of elements or
// every possible element, in which case it is named `wildcard`.

import (
	"bytes"
	"encoding/json"
	"slices"
)

// WildStrings represents a string set which can be a wildcard.
type WildStrings struct {
	Value []string
}

// WildPermissionDescs represents a PermissionDescriptor set which can be a wildcard.
type WildPermissionDescs struct {
	Value    []PermissionDesc
	Wildcard bool
}

// Contains checks if v is in the container.
func (c *WildStrings) Contains(v string) bool {
	if c.IsWildcard() {
		return true
	}
	return slices.Contains(c.Value, v)
}

// Contains checks if v is in the container.
func (c *WildPermissionDescs) Contains(v PermissionDesc) bool {
	if c.IsWildcard() {
		return true
	}
	return slices.ContainsFunc(c.Value, v.Equals)
}

// IsWildcard returns true iff the container is a wildcard.
func (c *WildStrings) IsWildcard() bool { return c.Value == nil }

// IsWildcard returns true iff the container is a wildcard.
func (c *WildPermissionDescs) IsWildcard() bool { return c.Wildcard }

// Restrict transforms the container into an empty one.
func (c *WildStrings) Restrict() { c.Value = []string{} }

// Restrict transforms the container into an empty one.
func (c *WildPermissionDescs) Restrict() {
	c.Value = []PermissionDesc{}
	c.Wildcard = false
}

// Add adds v to the container.
func (c *WildStrings) Add(v string) { c.Value = append(c.Value, v) }

// Add adds v to the container and converts container to non-wildcard (if it's still
// wildcard).
func (c *WildPermissionDescs) Add(v PermissionDesc) {
	c.Value = append(c.Value, v)
	c.Wildcard = false
}

// MarshalJSON implements the json.Marshaler interface.
func (c WildStrings) MarshalJSON() ([]byte, error) {
	if c.IsWildcard() {
		return []byte(`"*"`), nil
	}
	return json.Marshal(c.Value)
}

// MarshalJSON implements the json.Marshaler interface.
func (c WildPermissionDescs) MarshalJSON() ([]byte, error) {
	if c.IsWildcard() {
		return []byte(`"*"`), nil
	}
	return json.Marshal(c.Value)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
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

// UnmarshalJSON implements the json.Unmarshaler interface.
func (c *WildPermissionDescs) UnmarshalJSON(data []byte) error {
	c.Wildcard = bytes.Equal(data, []byte(`"*"`))
	if !c.Wildcard {
		us := []PermissionDesc{}
		if err := json.Unmarshal(data, &us); err != nil {
			return err
		}
		c.Value = us
	}
	return nil
}
