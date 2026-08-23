package machine

import (
	"regexp"
	"strings"
)

var validNewListName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func validateListName(name string) error {
	invalid := name == "" || strings.HasPrefix(name, ".") || strings.ContainsAny(name, "/\\\x00")
	if invalid {
		return apiError(CodeBadRequest, "list name must be a visible file basename")
	}
	return nil
}

func validateNewListName(name string) error {
	if err := validateListName(name); err != nil {
		return err
	}
	if !validNewListName.MatchString(name) {
		return apiError(CodeBadRequest,
			"new list names may contain only letters, numbers, hyphens, and underscores")
	}
	return nil
}

func taskListName(name string) (string, error) {
	if name == "" {
		name = ListToday
	}
	return name, validateListName(name)
}

func rejectUpdateCompletion(request Request) error {
	if request.Changes.Completed == nil {
		return nil
	}
	if request.Operation == OpTaskUpdate || request.Operation == OpSubtaskUpdate {
		return apiError(CodeBadRequest, "completion requires a setCompleted operation")
	}
	return nil
}
