package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/krisitan/tasks-go/internal/machine"
)

func runMachineCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "api":
		exitMachine(machine.RunAPI(store, args[1:], os.Stdin, os.Stdout))
		return true
	case "migrate":
		exitMachine(runMigration(args[1:]))
		return true
	default:
		return false
	}
}

func runMigration(args []string) error {
	if len(args) == 0 || args[0] != "ids" {
		return fmt.Errorf("usage: tasks migrate ids [--dry-run]")
	}
	dryRun := len(args) == 2 && args[1] == "--dry-run"
	if len(args) > 1 && !dryRun {
		return fmt.Errorf("usage: tasks migrate ids [--dry-run]")
	}
	report, err := store.MigrateIDs(dryRun)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}

func exitMachine(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
