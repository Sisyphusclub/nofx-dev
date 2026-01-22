package coinank

import (
	"os"
	"testing"
)

var TestApikey = "" //need fill the apikey before test

// skipInCI skips tests that require external network access when running in CI
func skipInCI(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test that requires external network access in CI environment")
	}
}
