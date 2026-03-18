package main

import (
	"fmt"
	"os"
)

const TMPFILE = "/tmp/metrics_lock"

func lock(force bool) error {
	_, err := os.Stat(TMPFILE)

	// 3 cases
	// Lock file exists and is read
	// Lock file failed to read
	// Lock file does not exist

	if err != nil && !os.IsNotExist(err) {
		// Failed to read lock file
		return err
	}

	if err == nil {
		if !force {
			return fmt.Errorf(
				"Lock file '%s' is currently held by another process. "+
					"Only remove it or run with --force if you are sure no other instance is running!",
				TMPFILE,
			)
		} else {
			return nil
		}
	}

	if os.IsNotExist(err) {
		// Create lock file
		_, err := os.Create(TMPFILE)
		return err
	}

	// Unreachable
	panic("Unreachable")
}

func unlock() error {
	return os.Remove(TMPFILE)
}
