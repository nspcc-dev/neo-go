package native

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The specification is following C# code:
// string domain = string.Join('.', name.Split('.')[^2..]);
func TestParseDomain(t *testing.T) {
	testCases := []struct {
		name   string
		domain string
	}{
		{"sim.pl.e", "pl.e"},
		{"some.long.d.o.m.a.i.n", "i.n"},
		{"t.wo", "t.wo"},
		{".dot", ".dot"},
		{".d.ot", "d.ot"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dom, ok := domainFromString(tc.name)
			require.True(t, ok)
			require.Equal(t, tc.domain, dom)
		})
	}

	_, ok := domainFromString("nodots")
	require.False(t, ok)
}

func TestNameService_CheckName(t *testing.T) {
	// tests are got from the C# implementation
	testCases := []struct {
		Type       RecordType
		Name       string
		ShouldFail bool
	}{
		{Type: RecordTypeA, Name: "0.0.0.0"},
		{Type: RecordTypeA, Name: "10.10.10.10"},
		{Type: RecordTypeA, Name: "255.255.255.255"},
		{Type: RecordTypeA, Name: "192.168.1.1"},
		{Type: RecordTypeA, Name: "1a", ShouldFail: true},
		{Type: RecordTypeA, Name: "256.0.0.0", ShouldFail: true},
		{Type: RecordTypeA, Name: "01.01.01.01", ShouldFail: true},
		{Type: RecordTypeA, Name: "00.0.0.0", ShouldFail: true},
		{Type: RecordTypeA, Name: "0.0.0.-1", ShouldFail: true},
		{Type: RecordTypeA, Name: "0.0.0.0.1", ShouldFail: true},
		{Type: RecordTypeA, Name: "11111111.11111111.11111111.11111111", ShouldFail: true},
		{Type: RecordTypeA, Name: "11111111.11111111.11111111.11111111", ShouldFail: true},
		{Type: RecordTypeA, Name: "ff.ff.ff.ff", ShouldFail: true},
		{Type: RecordTypeA, Name: "0.0.256", ShouldFail: true},
		{Type: RecordTypeA, Name: "0.0.0", ShouldFail: true},
		{Type: RecordTypeA, Name: "0.257", ShouldFail: true},
		{Type: RecordTypeA, Name: "1.1", ShouldFail: true},
		{Type: RecordTypeA, Name: "257", ShouldFail: true},
		{Type: RecordTypeA, Name: "1", ShouldFail: true},
		{Type: RecordTypeAAAA, Name: "2001:db8::8:800:200c:417a"},
		{Type: RecordTypeAAAA, Name: "ff01::101"},
		{Type: RecordTypeAAAA, Name: "::1"},
		{Type: RecordTypeAAAA, Name: "::"},
		{Type: RecordTypeAAAA, Name: "2001:db8:0:0:8:800:200c:417a"},
		{Type: RecordTypeAAAA, Name: "ff01:0:0:0:0:0:0:101"},
		{Type: RecordTypeAAAA, Name: "0:0:0:0:0:0:0:1"},
		{Type: RecordTypeAAAA, Name: "0:0:0:0:0:0:0:0"},
		{Type: RecordTypeAAAA, Name: "2001:DB8::8:800:200C:417A", ShouldFail: true},
		{Type: RecordTypeAAAA, Name: "FF01::101", ShouldFail: true},
		{Type: RecordTypeAAAA, Name: "fF01::101", ShouldFail: true},
		{Type: RecordTypeAAAA, Name: "2001:DB8:0:0:8:800:200C:417A", ShouldFail: true},
		{Type: RecordTypeAAAA, Name: "FF01:0:0:0:0:0:0:101", ShouldFail: true},
		{Type: RecordTypeAAAA, Name: "::ffff:1.01.1.01", ShouldFail: true},
		{Type: RecordTypeAAAA, Name: "2001:DB8:0:0:8:800:200C:4Z", ShouldFail: true},
		{Type: RecordTypeAAAA, Name: "::13.1.68.3", ShouldFail: true},
	}
	for _, testCase := range testCases {
		if testCase.ShouldFail {
			require.Panics(t, func() {
				checkName(testCase.Type, testCase.Name)
			})
		} else {
			require.NotPanics(t, func() {
				checkName(testCase.Type, testCase.Name)
			})
		}
	}
}
