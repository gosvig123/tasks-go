package machine

import (
	"fmt"
	"strings"
	"time"

	"github.com/krisitan/tasks-go/internal/task"
)

func applyChanges(item *task.Task, changes TaskChanges) error {
	if changes.Title != nil {
		title := strings.TrimSpace(*changes.Title)
		if title == "" {
			return apiError(CodeBadRequest, "title cannot be empty")
		}
		item.Content = title
	}
	if changes.Description != nil {
		item.Description = *changes.Description
	}
	if err := applySchedule(item, changes); err != nil {
		return err
	}
	if changes.Completed != nil {
		setCompleted(item, *changes.Completed)
	}
	return nil
}

func applySchedule(item *task.Task, changes TaskChanges) error {
	if changes.DueDate != nil {
		value, err := parseDate(*changes.DueDate)
		if err != nil {
			return err
		}
		item.DueDate = value
	}
	if changes.StartTime != nil {
		value, err := parseStartTime(*changes.StartTime)
		if err != nil {
			return err
		}
		item.StartTime = value
	}
	if changes.EstimateSeconds != nil {
		item.Estimate = duration(*changes.EstimateSeconds)
	}
	if changes.RecurrenceDays != nil {
		item.RecurDays = max(0, *changes.RecurrenceDays)
	}
	return nil
}

func parseDate(value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return nil, apiError(CodeBadRequest, "invalid due date %q", value)
	}
	return &parsed, nil
}

func parseStartTime(value string) (*task.TimeOfDay, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var hour, minute int
	if _, err := fmt.Sscanf(value, "%d:%d", &hour, &minute); err != nil {
		return nil, apiError(CodeBadRequest, "invalid start time %q", value)
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return nil, apiError(CodeBadRequest, "invalid start time %q", value)
	}
	return &task.TimeOfDay{Hour: hour, Minute: minute}, nil
}

func duration(seconds int) *time.Duration {
	if seconds <= 0 {
		return nil
	}
	value := time.Duration(seconds) * time.Second
	return &value
}
