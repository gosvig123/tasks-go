package machine

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/krisitan/tasks-go/internal/storage"
)

func RunAPI(store *storage.Storage, args []string, input io.Reader, output io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tasks api lists | tasks api snapshot --list today | tasks api exec")
	}
	service := &Service{Store: store}
	switch args[0] {
	case "lists":
		return runLists(service, args[1:], output)
	case "snapshot":
		return runSnapshot(service, args[1:], output)
	case "exec":
		return runExec(service, input, output)
	default:
		return fmt.Errorf("unknown api command %q", args[0])
	}
}

func runLists(service *Service, args []string, output io.Writer) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: tasks api lists")
	}
	lists, err := service.Lists()
	if err != nil {
		return normalizeError(err)
	}
	return encode(output, lists)
}

func runSnapshot(service *Service, args []string, output io.Writer) error {
	listName, err := snapshotList(args)
	if err != nil {
		return err
	}
	snapshot, err := service.Snapshot(listName)
	if err != nil {
		return normalizeError(err)
	}
	return encode(output, snapshot)
}

func snapshotList(args []string) (string, error) {
	if len(args) == 0 {
		return ListToday, nil
	}
	if len(args) == 2 && args[0] == "--list" && args[1] != "" {
		return args[1], nil
	}
	return "", fmt.Errorf("usage: tasks api snapshot --list <name>")
}

func runExec(service *Service, input io.Reader, output io.Writer) error {
	var request Request
	decoder := json.NewDecoder(input)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return encodeFailure(output, apiError(CodeBadRequest, "invalid request: %v", err))
	}
	snapshot, err := service.Execute(request)
	if err != nil {
		return encodeFailure(output, normalizeError(err))
	}
	return encode(output, Response{Success: true, Snapshot: snapshot})
}

func encodeFailure(output io.Writer, err error) error {
	return encode(output, Response{Success: false, Error: responseError(err)})
}

func encode(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func normalizeError(err error) error {
	if _, ok := err.(*Error); ok {
		return err
	}
	message := err.Error()
	if message == "task IDs missing; run 'tasks migrate ids --dry-run' then 'tasks migrate ids'" {
		return apiError(CodeMigrationRequired, "%s", message)
	}
	return err
}
