package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	TasksDir            string
	CurrentListFile     string
	LastResetFile       string
	DefaultList         string
	CleanupDays         int
	DayStartHour        int
	DayEndHour          int
	TimelineSlotMinutes int
}

func Default() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		TasksDir:            filepath.Join(home, "tasks-lists"),
		CurrentListFile:     filepath.Join(home, ".current-tasks-list"),
		LastResetFile:       filepath.Join(home, ".tasks-today-last-reset"),
		DefaultList:         "main",
		CleanupDays:         30,
		DayStartHour:        8,
		DayEndHour:          18,
		TimelineSlotMinutes: 30,
	}
}
