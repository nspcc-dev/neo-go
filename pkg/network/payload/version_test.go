package payload

import (
	"bytes"
	"reflect"
	"testing"
)

func TestVersionEncodeDecode(t *testing.T) {
	p := NewVersion(3000, "/NEO/", 0, true)

	buf := new(bytes.Buffer)
	p.Encode(buf)

	pd := &Version{}
	pd.Decode(buf)

	if !reflect.DeepEqual(p, pd) {
		t.Fatalf("expect %v to be equal to %v", p, pd)
	}
}
