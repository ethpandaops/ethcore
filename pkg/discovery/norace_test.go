//go:build !race

package discovery

import "testing"

// SkipIfRace is a no-op when the race detector is not enabled.
func SkipIfRace(_ *testing.T) {}
