package kurtosis

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// testLockDir is the path to the lock directory.
var testLockDir = filepath.Join(os.TempDir(), "ethcore-kurtosis-test.lock")

// AcquireTestLock acquires the global test lock for Kurtosis tests using directory creation.
// This should be called at the beginning of TestMain for test packages
// that use Kurtosis networks.
// Directory creation is atomic across processes.
func AcquireTestLock() {
	fmt.Printf("[PID %d] Attempting to acquire test lock at %s\n", os.Getpid(), testLockDir)

	// Try to acquire the lock with a timeout
	startTime := time.Now()
	timeout := 5 * time.Minute
	attempts := 0

	for {
		attempts++

		// Try to create the lock directory exclusively
		err := os.Mkdir(testLockDir, 0755)
		if err == nil {
			// We got the lock!
			fmt.Printf("[PID %d] Successfully acquired test lock after %d attempts\n", os.Getpid(), attempts)

			// Write our PID to a file in the directory
			pidFile := filepath.Join(testLockDir, "pid")

			f, _ := os.Create(pidFile)
			if f != nil {
				fmt.Fprintf(f, "PID: %d\nTime: %s\n", os.Getpid(), time.Now().Format(time.RFC3339))

				f.Close()
			}

			return
		}

		// Check if directory already exists
		if os.IsExist(err) {
			// Check if the lock is stale
			if info, statErr := os.Stat(testLockDir); statErr == nil {
				age := time.Since(info.ModTime())

				// Read PID file to see who owns it
				if attempts == 1 || attempts%50 == 0 {
					pidFile := filepath.Join(testLockDir, "pid")
					if data, readErr := os.ReadFile(pidFile); readErr == nil {
						fmt.Printf("[PID %d] Lock is held by another process: %s", os.Getpid(), string(data))
					} else {
						fmt.Printf("[PID %d] Lock directory exists (age: %v)\n", os.Getpid(), age)
					}
				}

				// If lock is older than 2 minutes, consider it stale
				if age > 2*time.Minute {
					fmt.Printf("[PID %d] Lock is stale (age: %v), removing it\n", os.Getpid(), age)

					os.RemoveAll(testLockDir)

					continue
				}
			}
		}

		// Check if we've timed out
		if time.Since(startTime) > timeout {
			panic(fmt.Sprintf("[PID %d] Failed to acquire test lock after %v", os.Getpid(), timeout))
		}

		// Wait before trying again
		time.Sleep(100 * time.Millisecond)
	}
}

// ReleaseTestLock releases the global test lock.
// This should be called at the end of TestMain (or in a defer).
func ReleaseTestLock() {
	fmt.Printf("[PID %d] Releasing test lock\n", os.Getpid())

	// Remove the lock directory
	os.RemoveAll(testLockDir)
}
