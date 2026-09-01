package main

import (
	"fmt"
	"os"
)

func runLocked(action func()) {
	err := store.WithLock(func() error {
		action()
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error locking task storage: %v\n", err)
		os.Exit(1)
	}
}
