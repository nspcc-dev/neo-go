package payload

import (
	"bytes"
	"reflect"
	"testing"
)

func TestVersionEncodeDecode(t *testing.T) {
	version := NewVersion(13337, 3000, "/NEO:0.0.1/", 0, true)

	buf := new(bytes.Buffer)
	if err := version.EncodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	if int(version.Size()) != buf.Len() {
		t.Fatalf("Expected version size of %d", buf.Len())
	}

	versionDecoded := &Version{}
	if err := versionDecoded.DecodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(version, versionDecoded) {
		t.Fatalf("expected both version payload to be equal: %+v and %+v", version, versionDecoded)
	}

}
