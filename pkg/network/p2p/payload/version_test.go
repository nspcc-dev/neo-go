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

	versionDecoded := &Version{}
	if err := versionDecoded.DecodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(version, versionDecoded) {
		t.Fatalf("expected both version payload to be equal: %+v and %+v", version, versionDecoded)
	}

	if version.Size() != uint32(minVersionSize+len(version.UserAgent)) {
		t.Fatalf("Expected version size of %d", minVersionSize+len(version.UserAgent))
	}
}
