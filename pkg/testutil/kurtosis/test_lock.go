package kurtosis

import "sync"

// globalTestMutex ensures only one Kurtosis test suite runs at a time
// This prevents port conflicts when running tests in parallel.
var globalTestMutex sync.Mutex

// AcquireTestLock acquires the global test lock for Kurtosis tests.
// This should be called at the beginning of TestMain for test packages
// that use Kurtosis networks.
func AcquireTestLock() {
	globalTestMutex.Lock()
}

// ReleaseTestLock releases the global test lock.
// This should be called at the end of TestMain (or in a defer).
func ReleaseTestLock() {
	globalTestMutex.Unlock()
}
