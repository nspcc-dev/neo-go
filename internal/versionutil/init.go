package versionutil

import "github.com/nspcc-dev/neo-go/pkg/config"

// TestVersion is a NeoGo version that should be used to keep all
// compiled NEFs the same from run to run for tests.
const TestVersion = "0.90.0-test"

// init sets config.Version to a dummy TestVersion value to keep contract NEFs
// consistent between test runs for those packages who import it. For test usage
// only!
func init() {
	config.Version = TestVersion
}
