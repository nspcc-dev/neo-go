package callflag

import (
	"encoding/json"
	"errors"
	"strings"
)

// CallFlag represents a call flag.
type CallFlag byte

// Default flags.
const (
	ReadStates CallFlag = 1 << iota
	WriteStates
	AllowCall
	AllowNotify

	States            = ReadStates | WriteStates
	ReadOnly          = ReadStates | AllowCall
	All               = States | AllowCall | AllowNotify
	NoneFlag CallFlag = 0
)

var flagString = map[CallFlag]string{
	ReadStates:  "ReadStates",
	WriteStates: "WriteStates",
	AllowCall:   "AllowCall",
	AllowNotify: "AllowNotify",
	States:      "States",
	ReadOnly:    "ReadOnly",
	All:         "All",
	NoneFlag:    "None",
}

// basicFlags are all flags except All and None. It's used to stringify CallFlag
// where its bits are matched against these values from the values with sets of bits
// to simple flags, which is important to produce proper string representation
// matching C# Enum handling.
var basicFlags = []CallFlag{ReadOnly, States, ReadStates, WriteStates, AllowCall, AllowNotify}

// FromString parses an input string and returns a corresponding CallFlag.
func FromString(s string) (CallFlag, error) {
	flags := strings.Split(s, ",")
	if len(flags) == 0 {
		return NoneFlag, errors.New("empty flags")
	}
	if len(flags) == 1 {
		for f, str := range flagString {
			if s == str {
				return f, nil
			}
		}
		return NoneFlag, errors.New("unknown flag")
	}

	var res CallFlag

	for _, flag := range flags {
		var knownFlag bool

		flag = strings.TrimSpace(flag)
		for _, f := range basicFlags {
			if flag == flagString[f] {
				res |= f
				knownFlag = true
				break
			}
		}
		if !knownFlag {
			return NoneFlag, errors.New("unknown/inappropriate flag")
		}
	}
	return res, nil
}

// Has returns true iff all bits set in cf are also set in f.
func (f CallFlag) Has(cf CallFlag) bool {
	return f&cf == cf
}

// String implements Stringer interface.
func (f CallFlag) String() string {
	if flagString[f] != "" {
		return flagString[f]
	}

	var res string

	for _, flag := range basicFlags {
		if f.Has(flag) {
			if len(res) != 0 {
				res += ", "
			}
			res += flagString[flag]
			f &= ^flag // Some "States" shouldn't be combined with "ReadStates".
		}
	}
	return res
}

// MarshalJSON implements the json.Marshaler interface.
func (f CallFlag) MarshalJSON() ([]byte, error) {
	return []byte(`"` + f.String() + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (f *CallFlag) UnmarshalJSON(data []byte) error {
	var js string
	if err := json.Unmarshal(data, &js); err != nil {
		return err
	}
	flag, err := FromString(js)
	if err != nil {
		return err
	}
	*f = flag
	return nil
}

// MarshalYAML implements the YAML marshaler interface.
func (f CallFlag) MarshalYAML() (any, error) {
	return f.String(), nil
}

// UnmarshalYAML implements the YAML unmarshaler interface.
func (f *CallFlag) UnmarshalYAML(unmarshal func(any) error) error {
	var s string

	err := unmarshal(&s)
	if err != nil {
		return err
	}

	*f, err = FromString(s)
	return err
}
