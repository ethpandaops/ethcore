package kurtosis

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// testLockFile is the path to the lock file.
var testLockFile = filepath.Join(os.TempDir(), "ethcore-kurtosis-test.lock")

// AcquireTestLock acquires the global test lock for Kurtosis tests.
// This should be called at the beginning of TestMain for test packages
// that use Kurtosis networks.
// It uses a file-based lock to work across different test processes.
func AcquireTestLock() {
	// Try to acquire the lock with a timeout
	startTime := time.Now()
	timeout := 5 * time.Minute

	fmt.Printf("[PID %d] Attempting to acquire test lock at %s\n", os.Getpid(), testLockFile)

	for {
		// Try to create the lock file exclusively
		file, err := os.OpenFile(testLockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			fmt.Printf("[PID %d] Successfully acquired test lock\n", os.Getpid())

			_, werr := fmt.Fprintf(file, "PID: %d\n", os.Getpid())
			if werr != nil {
				fmt.Printf("[PID %d] Failed to write test lock\n", os.Getpid())
			}

			file.Close()

			return
		}

		// Check if we've timed out
		if time.Since(startTime) > timeout {
			panic(fmt.Sprintf("Failed to acquire test lock after %v - another test may be stuck", timeout))
		}

		// Check if the lock file is stale (older than 5 minutes)
		if info, err := os.Stat(testLockFile); err == nil {
			if time.Since(info.ModTime()) > 5*time.Minute {
				fmt.Printf("[PID %d] Lock file is stale, removing it\n", os.Getpid())

				// Lock file is stale, try to remove it
				os.Remove(testLockFile)
			}
		}

		// Wait a bit before trying again
		time.Sleep(100 * time.Millisecond)
	}
}

// ReleaseTestLock releases the global test lock.
// This should be called at the end of TestMain (or in a defer).
func ReleaseTestLock() {
	fmt.Printf("[PID %d] Releasing test lock\n", os.Getpid())
	os.Remove(testLockFile)
}
