//go:build race

package discovery

import "testing"

// SkipIfRace skips the test when running with the race detector enabled.
func SkipIfRace(t *testing.T) {
	t.Helper()
	t.Skip("skipping under race detector")
}
